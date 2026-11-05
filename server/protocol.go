package main

import "encoding/binary"

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

func encodeWindowSize(cols, rows uint16) []byte {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint16(payload[0:2], cols)
	binary.BigEndian.PutUint16(payload[2:4], rows)
	return payload
}

func decodeWindowSize(payload []byte) (cols, rows uint16, ok bool) {
	if len(payload) != 4 {
		return 0, 0, false
	}
	cols = binary.BigEndian.Uint16(payload[0:2])
	rows = binary.BigEndian.Uint16(payload[2:4])
	if cols == 0 || rows == 0 {
		return 0, 0, false
	}
	return cols, rows, true
}
