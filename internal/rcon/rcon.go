package rcon

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const (
	packetTypeCommand = 2
	packetTypeLogin   = 3
)

func SendCommand(host string, port int, password, command string, timeout time.Duration) (string, error) {
	addr := net.JoinHostPort(host, fmt.Sprint(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", fmt.Errorf("rcon dial %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", fmt.Errorf("rcon set deadline: %w", err)
	}

	if err := writePacket(conn, 0, packetTypeLogin, password); err != nil {
		return "", fmt.Errorf("rcon login: %w", err)
	}

	resp, _, err := readPacket(conn)
	if err != nil {
		return "", fmt.Errorf("rcon login response: %w", err)
	}
	if resp == "" {
		return "", fmt.Errorf("rcon: login failed (wrong password?)")
	}

	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return "", fmt.Errorf("rcon set deadline: %w", err)
	}

	if err := writePacket(conn, 1, packetTypeCommand, command); err != nil {
		return "", fmt.Errorf("rcon command: %w", err)
	}

	resp, reqID, err := readPacket(conn)
	if err != nil {
		return "", fmt.Errorf("rcon response: %w", err)
	}
	if reqID != 1 {
		return resp, fmt.Errorf("rcon: unexpected request ID %d (expected 1)", reqID)
	}

	return resp, nil
}

func writePacket(w io.Writer, reqID int32, pktType int32, payload string) error {
	body := []byte(payload)
	pktLen := int32(4 + 4 + len(body) + 2)

	buf := make([]byte, 4+4+4+len(body)+2)
	binary.LittleEndian.PutUint32(buf[0:4], uint32(pktLen))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(reqID))
	binary.LittleEndian.PutUint32(buf[8:12], uint32(pktType))
	copy(buf[12:], body)

	_, err := w.Write(buf)
	return err
}

func readPacket(r io.Reader) (body string, reqID int32, err error) {
	var length int32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", 0, fmt.Errorf("read length: %w", err)
	}
	if length < 10 {
		return "", 0, fmt.Errorf("packet too short: %d bytes", length)
	}

	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", 0, fmt.Errorf("read payload: %w", err)
	}

	reqID = int32(binary.LittleEndian.Uint32(buf[0:4]))
	pktType := int32(binary.LittleEndian.Uint32(buf[4:8]))
	_ = pktType

	bodyBytes := buf[8:]
	if len(bodyBytes) >= 2 {
		bodyBytes = bodyBytes[:len(bodyBytes)-2]
	}
	return string(bodyBytes), reqID, nil
}
