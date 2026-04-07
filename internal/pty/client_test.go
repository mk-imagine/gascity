//go:build !windows

package pty

import (
	"bytes"
	"io"
	"log"
	"strings"
	"testing"
)

// TestNewClient_ValidAddr_ReturnsClient verifies that NewClient returns a
// non-nil *Client and a nil error when given a non-empty address and a
// zero-value ClientOptions.
func TestNewClient_ValidAddr_ReturnsClient(t *testing.T) {
	c, err := NewClient("localhost:8080", ClientOptions{})
	if err != nil {
		t.Fatalf("NewClient(\"localhost:8080\", ClientOptions{}) returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient(\"localhost:8080\", ClientOptions{}) returned nil *Client, want non-nil")
	}
}

// TestNewClient_CustomOptions_Accepted verifies that NewClient accepts custom
// Stdin, Stdout, and Raw values and returns a non-nil *Client with nil error.
func TestNewClient_CustomOptions_Accepted(t *testing.T) {
	customReader := strings.NewReader("input")
	customWriter := &bytes.Buffer{}

	c, err := NewClient("localhost:8080", ClientOptions{
		Raw:    true,
		Stdin:  customReader,
		Stdout: customWriter,
	})
	if err != nil {
		t.Fatalf("NewClient with custom options returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with custom options returned nil *Client, want non-nil")
	}
}

// TestNewClient_CustomLogger_Accepted verifies that NewClient accepts a custom
// Logger and returns a non-nil *Client with nil error.
func TestNewClient_CustomLogger_Accepted(t *testing.T) {
	var buf bytes.Buffer
	customLogger := log.New(&buf, "test: ", 0)

	c, err := NewClient("127.0.0.1:9999", ClientOptions{
		Logger: customLogger,
	})
	if err != nil {
		t.Fatalf("NewClient(\"127.0.0.1:9999\", ClientOptions{Logger: ...}) returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient(\"127.0.0.1:9999\", ClientOptions{Logger: ...}) returned nil *Client, want non-nil")
	}
}

// TestNewClient_EmptyAddr_ReturnsError verifies that NewClient returns a nil
// *Client and a non-nil error whose message contains "addr" when given an
// empty address string.
func TestNewClient_EmptyAddr_ReturnsError(t *testing.T) {
	c, err := NewClient("", ClientOptions{})
	if err == nil {
		t.Fatal("NewClient(\"\", ClientOptions{}) returned nil error, want error containing \"addr\"")
	}
	if !strings.Contains(err.Error(), "addr") {
		t.Fatalf("NewClient(\"\", ClientOptions{}) error = %q, want it to contain \"addr\"", err.Error())
	}
	if c != nil {
		t.Fatalf("NewClient(\"\", ClientOptions{}) returned non-nil *Client, want nil")
	}
}

// TestNewClient_NilDefaults_Accepted verifies that NewClient accepts nil
// Stdin, nil Stdout, and nil Logger (defaults should be applied internally)
// and returns a non-nil *Client with nil error.
func TestNewClient_NilDefaults_Accepted(t *testing.T) {
	c, err := NewClient("localhost:8080", ClientOptions{
		Stdin:  nil,
		Stdout: nil,
		Logger: nil,
	})
	if err != nil {
		t.Fatalf("NewClient with nil optional fields returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with nil optional fields returned nil *Client, want non-nil")
	}
}

// TestNewClient_AddressStoredAsIs verifies that NewClient accepts addresses
// with and without a scheme prefix, returning a non-nil *Client and nil error
// in both cases.
func TestNewClient_AddressStoredAsIs(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "with ws scheme",
			addr: "ws://localhost:8080",
		},
		{
			name: "without scheme",
			addr: "localhost:8080",
		},
		{
			name: "IP without scheme",
			addr: "127.0.0.1:9999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewClient(tt.addr, ClientOptions{})
			if err != nil {
				t.Fatalf("NewClient(%q, ClientOptions{}) returned unexpected error: %v", tt.addr, err)
			}
			if c == nil {
				t.Fatalf("NewClient(%q, ClientOptions{}) returned nil *Client, want non-nil", tt.addr)
			}
		})
	}
}

// TestNewClient_WriterInterface_Accepted verifies that NewClient accepts any
// io.Writer as Stdout (not just *os.File).
func TestNewClient_WriterInterface_Accepted(t *testing.T) {
	var buf bytes.Buffer
	c, err := NewClient("localhost:8080", ClientOptions{
		Stdout: io.Writer(&buf),
	})
	if err != nil {
		t.Fatalf("NewClient with io.Writer Stdout returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with io.Writer Stdout returned nil *Client, want non-nil")
	}
}

// TestNewClient_ReaderInterface_Accepted verifies that NewClient accepts any
// io.Reader as Stdin (not just *os.File).
func TestNewClient_ReaderInterface_Accepted(t *testing.T) {
	r := strings.NewReader("some input")
	c, err := NewClient("localhost:8080", ClientOptions{
		Stdin: io.Reader(r),
	})
	if err != nil {
		t.Fatalf("NewClient with io.Reader Stdin returned unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("NewClient with io.Reader Stdin returned nil *Client, want non-nil")
	}
}
