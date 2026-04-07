//go:build !windows

package pty

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestEncodeResize_PositiveCases verifies that EncodeResize produces valid JSON
// with the correct type, rows, and cols fields for representative inputs.
func TestEncodeResize_PositiveCases(t *testing.T) {
	tests := []struct {
		name     string
		rows     uint16
		cols     uint16
		wantJSON string
	}{
		{
			name:     "standard terminal size",
			rows:     24,
			cols:     80,
			wantJSON: `{"type":"resize","rows":24,"cols":80}`,
		},
		{
			name:     "zero dimensions",
			rows:     0,
			cols:     0,
			wantJSON: `{"type":"resize","rows":0,"cols":0}`,
		},
		{
			name:     "max uint16 dimensions",
			rows:     65535,
			cols:     65535,
			wantJSON: `{"type":"resize","rows":65535,"cols":65535}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncodeResize(tt.rows, tt.cols)
			if err != nil {
				t.Fatalf("EncodeResize(%d, %d) returned unexpected error: %v", tt.rows, tt.cols, err)
			}
			if string(got) != tt.wantJSON {
				t.Fatalf("EncodeResize(%d, %d) = %q, want %q", tt.rows, tt.cols, got, tt.wantJSON)
			}
		})
	}
}

// TestEncodeResize_TypeFieldAlwaysResize verifies that the "type" field in the
// encoded output is always "resize" regardless of the row/col input values.
func TestEncodeResize_TypeFieldAlwaysResize(t *testing.T) {
	inputs := [][2]uint16{
		{0, 0},
		{24, 80},
		{65535, 65535},
		{1, 1},
	}

	for _, pair := range inputs {
		rows, cols := pair[0], pair[1]
		data, err := EncodeResize(rows, cols)
		if err != nil {
			t.Fatalf("EncodeResize(%d, %d) returned unexpected error: %v", rows, cols, err)
		}

		var msg struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("EncodeResize(%d, %d) output is not valid JSON: %v", rows, cols, err)
		}
		if msg.Type != "resize" {
			t.Fatalf("EncodeResize(%d, %d) type = %q, want %q", rows, cols, msg.Type, "resize")
		}
	}
}

// TestDecodeResize_PositiveCases verifies that DecodeResize correctly
// deserializes valid resize JSON into a ResizeMessage.
func TestDecodeResize_PositiveCases(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want ResizeMessage
	}{
		{
			name: "standard terminal size",
			data: []byte(`{"type":"resize","rows":24,"cols":80}`),
			want: ResizeMessage{Type: "resize", Rows: 24, Cols: 80},
		},
		{
			name: "zero dimensions",
			data: []byte(`{"type":"resize","rows":0,"cols":0}`),
			want: ResizeMessage{Type: "resize", Rows: 0, Cols: 0},
		},
		{
			name: "max uint16 dimensions",
			data: []byte(`{"type":"resize","rows":65535,"cols":65535}`),
			want: ResizeMessage{Type: "resize", Rows: 65535, Cols: 65535},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeResize(tt.data)
			if err != nil {
				t.Fatalf("DecodeResize(%q) returned unexpected error: %v", tt.data, err)
			}
			if got != tt.want {
				t.Fatalf("DecodeResize(%q) = %+v, want %+v", tt.data, got, tt.want)
			}
		})
	}
}

// TestRoundTrip verifies that DecodeResize(EncodeResize(rows, cols)) preserves
// values with a nil error for representative and boundary inputs.
func TestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		rows uint16
		cols uint16
	}{
		{name: "standard", rows: 24, cols: 80},
		{name: "zero", rows: 0, cols: 0},
		{name: "max uint16", rows: 65535, cols: 65535},
		{name: "asymmetric", rows: 50, cols: 200},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := EncodeResize(tt.rows, tt.cols)
			if err != nil {
				t.Fatalf("EncodeResize(%d, %d) error: %v", tt.rows, tt.cols, err)
			}

			decoded, err := DecodeResize(encoded)
			if err != nil {
				t.Fatalf("DecodeResize(EncodeResize(%d, %d)) error: %v", tt.rows, tt.cols, err)
			}

			if decoded.Type != "resize" {
				t.Errorf("round-trip Type = %q, want %q", decoded.Type, "resize")
			}
			if decoded.Rows != tt.rows {
				t.Errorf("round-trip Rows = %d, want %d", decoded.Rows, tt.rows)
			}
			if decoded.Cols != tt.cols {
				t.Errorf("round-trip Cols = %d, want %d", decoded.Cols, tt.cols)
			}
		})
	}
}

// TestDecodeResize_NegativeCases verifies that DecodeResize returns an error
// for invalid, malformed, or type-mismatched input.
func TestDecodeResize_NegativeCases(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		errContains string // optional substring the error message should contain
	}{
		{
			name:        "not json",
			data:        []byte("not json"),
			errContains: "",
		},
		{
			name:        "unknown type field",
			data:        []byte(`{"type":"unknown"}`),
			errContains: "unrecognized",
		},
		{
			name: "nil input",
			data: nil,
		},
		{
			name: "empty input",
			data: []byte{},
		},
		{
			name:        "missing type field (zero value is not resize)",
			data:        []byte(`{"rows":24,"cols":80}`),
			errContains: "unrecognized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeResize(tt.data)
			if err == nil {
				t.Fatalf("DecodeResize(%q) returned nil error, want error; got %+v", tt.data, got)
			}
			if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("DecodeResize(%q) error = %q, want to contain %q", tt.data, err.Error(), tt.errContains)
			}
		})
	}
}
