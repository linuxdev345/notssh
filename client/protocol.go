package main

import (
	cryptorand "crypto/rand"
	"encoding/binary"
)

const (
	headerSize = 1 + 2

	kindHello    = 1
	kindHelloAck = 2
	kindInput    = 3
	kindOutput   = 4
	kindClose    = 5
)

func encodeFrame(maxPayload int, sessionID uint16, kind byte, payload []byte) []byte {
	if len(payload) > maxPayload {
		payload = payload[:maxPayload]
	}
	packet := make([]byte, headerSize+len(payload))
	packet[0] = kind
	binary.BigEndian.PutUint16(packet[1:3], sessionID)
	copy(packet[headerSize:], payload)
	return packet
}

func decodeFrame(maxPayload int, packet []byte) (uint16, byte, []byte, bool) {
	maxDatagram := headerSize + maxPayload
	if len(packet) < headerSize || len(packet) > maxDatagram {
		return 0, 0, nil, false
	}
	return binary.BigEndian.Uint16(packet[1:3]), packet[0], packet[headerSize:], true
}

func newSessionID() (uint16, error) {
	var data [2]byte
	if _, err := cryptorand.Read(data[:]); err != nil {
		return 0, err
	}
	sessionID := binary.BigEndian.Uint16(data[:])
	if sessionID == 0 {
		sessionID = 1
	}
	return sessionID, nil
}
