// notssh server.
package main

import (
	"flag"
	"log"
	"net"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/creack/pty"
)

func main() {
	configPath := flag.String("config", "config.toml", "TOML config path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	maxPayload := cfg.UDP.MaxPayload
	maxDatagram := headerSize + maxPayload

	readUDP, err := openUDPRead(cfg.UDP.ReadAddr)
	if err != nil {
		log.Fatalf("open UDP read endpoint: %v", err)
	}
	defer readUDP.Close()
	writeUDP, err := openUDPWrite(cfg.UDP.WriteAddr)
	if err != nil {
		log.Fatalf("open UDP write endpoint: %v", err)
	}
	defer writeUDP.Close()

	var writeMu sync.Mutex
	sendPacket := func(packet []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err := writeUDP.Write(packet)
		return err
	}

	log.Printf("[*] UDP boundary read=%s write=%s", cfg.UDP.ReadAddr, cfg.UDP.WriteAddr)
	connected, sessionID := waitForClient(readUDP, writeUDP, maxPayload, maxDatagram)
	if !connected {
		return
	}
	var activeSession atomic.Uint32
	activeSession.Store(uint32(sessionID))
	loadSessionID := func() uint16 {
		return uint16(activeSession.Load())
	}
	var lastHelloAckAt time.Time

	cmd := exec.Command("bash", "-il")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		log.Fatalf("start PTY: %v", err)
	}
	defer func() {
		_ = ptmx.Close()
		_ = cmd.Process.Kill()
	}()

	sendOutput := func(payload []byte) {
		for len(payload) > 0 {
			n := len(payload)
			if n > maxPayload {
				n = maxPayload
			}
			// Each PTY read is a delta; forward it without screen snapshots
			// or retransmission at this layer.
			packet := encodeFrame(maxPayload, loadSessionID(), kindOutput, payload[:n])
			if err := sendPacket(packet); err != nil {
				log.Printf("send output: %v", err)
			}
			payload = payload[n:]
		}
	}
	sendClose := func() {
		_ = sendPacket(encodeFrame(maxPayload, loadSessionID(), kindClose, nil))
	}
	go func() {
		_ = cmd.Wait()
		sendClose()
	}()
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := ptmx.Read(buf)
			if err != nil {
				sendClose()
				return
			}
			if n > 0 {
				sendOutput(buf[:n])
			}
		}
	}()

	packet := make([]byte, maxDatagram)
	for {
		n, err := readUDP.Read(packet)
		if err != nil {
			log.Printf("read UDP boundary: %v", err)
			return
		}
		frameSession, kind, payload, ok := decodeFrame(maxPayload, packet[:n])
		if !ok {
			continue
		}
		switch kind {
		case kindHello:
			currentSession := loadSessionID()
			if frameSession != currentSession {
				if frameSession == 0 {
					continue
				}
				activeSession.Store(uint32(frameSession))
				lastHelloAckAt = time.Time{}
				log.Printf("[*] transport session replaced: %d", frameSession)
			}
			if time.Since(lastHelloAckAt) < time.Second {
				continue
			}
			lastHelloAckAt = time.Now()
			_ = sendPacket(encodeFrame(maxPayload, loadSessionID(), kindHelloAck, nil))
		case kindInput:
			if frameSession != loadSessionID() {
				continue
			}
			if _, err := ptmx.Write(payload); err != nil {
				log.Printf("write PTY: %v", err)
				return
			}
		case kindClose:
			if frameSession == loadSessionID() {
				log.Println("Client not connected")
				activeSession.Store(0)
			}
		}
	}
}

func waitForClient(readUDP, writeUDP *udpEndpoint, maxPayload, maxDatagram int) (bool, uint16) {
	packet := make([]byte, maxDatagram)
	// The UDP relay is transparent: a read timeout means that no client hello
	// arrived, but does not prove that the relay itself has stopped.
	log.Println("Client not connected")
	for {
		_ = readUDP.conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err := readUDP.Read(packet)
		if err != nil {
			if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
				log.Println("Client not connected")
				continue
			}
			log.Printf("wait for client: %v", err)
			return false, 0
		}
		sessionID, kind, _, ok := decodeFrame(maxPayload, packet[:n])
		if !ok || sessionID == 0 || kind != kindHello {
			continue
		}
		if _, err := writeUDP.Write(encodeFrame(maxPayload, sessionID, kindHelloAck, nil)); err != nil {
			log.Printf("send transport ack: %v", err)
			continue
		}
		_ = readUDP.conn.SetReadDeadline(time.Time{})
		log.Println("transport connected")
		return true, sessionID
	}
}
