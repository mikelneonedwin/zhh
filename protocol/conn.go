package protocol

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

func ReadMessage(conn net.Conn) (*Message, error) {
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, fmt.Errorf("read length: %w", err)
	}
	if length > 100*1024*1024 {
		return nil, fmt.Errorf("message too large: %d bytes", length)
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, fmt.Errorf("read payload: %w", err)
	}
	var msg Message
	if err := json.Unmarshal(buf, &msg); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &msg, nil
}

func WriteMessage(conn net.Conn, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	if len(data) > 100*1024*1024 {
		return fmt.Errorf("message too large: %d bytes", len(data))
	}
	length := uint32(len(data))
	if err := binary.Write(conn, binary.BigEndian, length); err != nil {
		return fmt.Errorf("write length: %w", err)
	}
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("write payload: %w", err)
	}
	return nil
}

func SendMessageType(conn net.Conn, msgType string) error {
	return WriteMessage(conn, NewMessage(msgType, nil))
}

func ReadPayload[T any](conn net.Conn) (*T, error) {
	msg, err := ReadMessage(conn)
	if err != nil {
		return nil, err
	}
	var payload T
	if msg.Payload != nil {
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			return nil, fmt.Errorf("unmarshal payload: %w", err)
		}
	}
	return &payload, nil
}
