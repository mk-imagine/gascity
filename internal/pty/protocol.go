//go:build !windows

// Package pty provides WebSocket message protocol types and codec functions
// for communicating PTY events between a terminal host and a browser client.
package pty

import (
	"encoding/json"
	"fmt"
)

// ResizeMessage represents a terminal resize event sent over a WebSocket
// connection. The Type field is always "resize".
type ResizeMessage struct {
	Type string `json:"type"` // message type discriminator; always "resize"
	Rows uint16 `json:"rows"` // terminal row count
	Cols uint16 `json:"cols"` // terminal column count
}

// EncodeResize serializes a terminal resize event to JSON bytes. It sets
// the Type field to "resize" and encodes the given row and column counts.
// It never returns an error for valid uint16 input.
func EncodeResize(rows, cols uint16) ([]byte, error) {
	msg := ResizeMessage{
		Type: "resize",
		Rows: rows,
		Cols: cols,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("encoding resize message: %w", err)
	}
	return data, nil
}

// DecodeResize deserializes JSON bytes into a ResizeMessage. It validates
// that the type field is "resize". Returns a zero ResizeMessage and a
// non-nil error if the data is not valid JSON or the type is unrecognized.
func DecodeResize(data []byte) (ResizeMessage, error) {
	var msg ResizeMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ResizeMessage{}, fmt.Errorf("decoding resize message: %w", err)
	}
	if msg.Type != "resize" {
		return ResizeMessage{}, fmt.Errorf("decoding resize message: unrecognized type %q", msg.Type)
	}
	return msg, nil
}
