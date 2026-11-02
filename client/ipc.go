package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const ipcHeaderSize = 4

type udpEndpoint struct {
	conn *net.UDPConn
}

func openUDPRead(addr string) (*udpEndpoint, error) {
	local, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", local)
	if err != nil {
		return nil, err
	}
	return &udpEndpoint{conn: conn}, nil
}

func openUDPWrite(addr string) (*udpEndpoint, error) {
	remote, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		return nil, err
	}
	return &udpEndpoint{conn: conn}, nil
}

func (e *udpEndpoint) Read(p []byte) (int, error) {
	packet := make([]byte, ipcHeaderSize+len(p))
	n, err := e.conn.Read(packet)
	if err != nil {
		return 0, err
	}
	if n < ipcHeaderSize {
		return 0, fmt.Errorf("transport packet is shorter than its IPC header: %d bytes", n)
	}
	length := int(binary.BigEndian.Uint32(packet[:ipcHeaderSize]))
	if length != n-ipcHeaderSize {
		return 0, fmt.Errorf("transport packet length is %d, datagram contains %d bytes", length, n-ipcHeaderSize)
	}
	if length > len(p) {
		return 0, io.ErrShortBuffer
	}
	copy(p, packet[ipcHeaderSize:])
	return length, nil
}

func (e *udpEndpoint) Write(p []byte) (int, error) {
	packet := make([]byte, ipcHeaderSize+len(p))
	binary.BigEndian.PutUint32(packet[:ipcHeaderSize], uint32(len(p)))
	copy(packet[ipcHeaderSize:], p)
	n, err := e.conn.Write(packet)
	if err != nil {
		return 0, err
	}
	if n != len(packet) {
		return 0, io.ErrShortWrite
	}
	return n, nil
}

func (e *udpEndpoint) Close() error { return e.conn.Close() }
