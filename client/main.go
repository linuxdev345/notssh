// notssh client.
package main

import (
	"bytes"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/term"
)

func main() {
	configPath := flag.String("config", "config.toml", "TOML config path")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	maxPayload := cfg.UDP.MaxPayload
	connectionTimeout := time.Duration(cfg.UDP.ConnectionTimeoutSeconds) * time.Second
	maxDatagram := headerSize + maxPayload
	initialSessionID, err := newSessionID()
	if err != nil {
		log.Fatalf("create session ID: %v", err)
	}
	var sessionID atomic.Uint32
	sessionID.Store(uint32(initialSessionID))
	loadSessionID := func() uint16 {
		return uint16(sessionID.Load())
	}

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

	if !term.IsTerminal(int(os.Stdin.Fd())) {
		log.Fatal("client must be run in an interactive terminal")
	}
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("make terminal raw: %v", err)
	}
	terminal := &terminalOutput{file: os.Stdout}
	log.SetOutput(&terminalLogWriter{terminal: terminal})
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	defer func() {
		restoreTerminal(int(os.Stdin.Fd()), oldState)
		_, _ = writeUDP.Write(encodeFrame(maxPayload, loadSessionID(), kindClose, nil))
		_, _ = terminal.Write([]byte("\r\n[*] Session finished.\r\n"))
	}()
	_, _ = terminal.Write([]byte("Connecting..."))

	var connected atomic.Bool
	var lastRx atomic.Int64
	lastRx.Store(time.Now().UnixNano())
	serverClosed := make(chan struct{}, 1)
	var writeMu sync.Mutex
	sendPacket := func(packet []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err := writeUDP.Write(packet)
		return err
	}
	sendHello := func() error {
		return sendPacket(encodeFrame(maxPayload, loadSessionID(), kindHello, nil))
	}
	resetSession := func() {
		newID, err := newSessionID()
		if err != nil {
			log.Printf("create reset session ID: %v", err)
			return
		}
		sessionID.Store(uint32(newID))
		connected.Store(false)
		lastRx.Store(time.Now().UnixNano())
		log.Printf("transport session reset: %d", newID)
		if err := sendHello(); err != nil {
			log.Printf("send reset handshake: %v", err)
		}
	}
	markConnected := func() {
		if !connected.CompareAndSwap(false, true) {
			return
		}
		log.Println("transport connected")
	}

	go func() {
		buf := make([]byte, maxDatagram)
		for {
			n, err := readUDP.Read(buf)
			if err != nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			frameSession, kind, payload, ok := decodeFrame(maxPayload, buf[:n])
			if !ok || frameSession != loadSessionID() {
				continue
			}
			lastRx.Store(time.Now().UnixNano())
			switch kind {
			case kindHelloAck:
				markConnected()
			case kindOutput:
				_, _ = terminal.Write(payload)
			case kindClose:
				log.Println("server closed session")
				serverClosed <- struct{}{}
				return
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !connected.Load() || time.Since(time.Unix(0, lastRx.Load())) > connectionTimeout {
					if connected.Swap(false) {
						log.Println("Server not connected; restarting handshake")
					} else {
						log.Println("Server not connected")
					}
					if err := sendHello(); err != nil {
						log.Printf("send hello: %v", err)
					}
					continue
				}
				if err := sendHello(); err != nil {
					log.Printf("send handshake: %v", err)
				}
			}
		}
	}()

	if err := sendHello(); err != nil {
		log.Printf("send initial handshake: %v", err)
	}

	inputCh := make(chan []byte, 16)
	inputErr := make(chan error, 1)
	go func() {
		buf := make([]byte, maxPayload)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				inputErr <- err
				return
			}
			if n > 0 {
				data := append([]byte(nil), buf[:n]...)
				inputCh <- data
			}
		}
	}()

	for {
		select {
		case <-signals:
			return
		case <-serverClosed:
			return
		case <-inputErr:
			return
		case data := <-inputCh:
			if bytes.Contains(data, []byte{3}) {
				// Send Ctrl-C to the remote PTY and terminate the client.
				if connected.Load() {
					if err := sendPacket(encodeFrame(maxPayload, loadSessionID(), kindInput, data)); err != nil {
						log.Printf("send Ctrl-C: %v", err)
					}
				}
				return
			}
			if bytes.Contains(data, []byte{18}) {
				// Ctrl-R resets the local session and must not reach the remote PTY.
				resetSession()
				continue
			}
			if !connected.Load() {
				continue
			}
			if err := sendPacket(encodeFrame(maxPayload, loadSessionID(), kindInput, data)); err != nil {
				log.Printf("send input: %v", err)
				connected.Store(false)
				if helloErr := sendHello(); helloErr != nil {
					log.Printf("send handshake: %v", helloErr)
				}
			}
		}
	}
}

func restoreTerminal(fd int, state *term.State) {
	_ = term.Restore(fd, state)
	// Fallback for a terminal whose state was not restored after a debugger stop
	// or a forced signal.
	stty := exec.Command("stty", "sane")
	stty.Stdin = os.Stdin
	stty.Stdout = os.Stdout
	stty.Stderr = os.Stderr
	_ = stty.Run()
}

type terminalOutput struct {
	mu   sync.Mutex
	file *os.File
}

func (output *terminalOutput) Write(payload []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.file.Write(payload)
}

type terminalLogWriter struct {
	terminal *terminalOutput
}

func (writer *terminalLogWriter) Write(payload []byte) (int, error) {
	text := strings.ReplaceAll(string(payload), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	if !strings.HasPrefix(text, "\r\n") {
		text = "\r\n" + text
	}
	if _, err := writer.terminal.Write([]byte(text)); err != nil {
		return 0, err
	}
	return len(payload), nil
}
