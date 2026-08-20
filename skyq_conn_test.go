package main

import (
	"io"
	"net"
	"testing"
	"time"
)

// TestSendSkyCommandOverConn_HandshakeAndCommandFraming drives the client
// side of sendSkyCommandOverConn against an in-memory net.Pipe(), with this
// test playing the role of the Sky Q box: one handshake round (a chunk
// under 24 bytes, echoed back), then a chunk of 24+ bytes to signal the
// handshake is complete, at which point the client must send the two
// command packets described in skyq.go's comments.
func TestSendSkyCommandOverConn_HandshakeAndCommandFraming(t *testing.T) {
	client, box := net.Pipe()
	defer client.Close()
	defer box.Close()

	const code = 6 // channel-up
	errCh := make(chan error, 1)
	go func() {
		errCh <- sendSkyCommandOverConn(client, code)
	}()

	deadline := time.Now().Add(2 * time.Second)
	client.SetDeadline(deadline)
	box.SetDeadline(deadline)

	// Round 1: box sends a 12-byte handshake chunk (n < 24), client should
	// echo exactly those 12 bytes back (l starts at 12).
	handshakeChunk := []byte("hello sky box")[:12]
	if _, err := box.Write(handshakeChunk); err != nil {
		t.Fatalf("box write handshake chunk: %v", err)
	}
	echoed := make([]byte, 12)
	if _, err := io.ReadFull(box, echoed); err != nil {
		t.Fatalf("box read echo: %v", err)
	}
	if string(echoed) != string(handshakeChunk) {
		t.Errorf("echoed handshake = %q, want %q", echoed, handshakeChunk)
	}

	// Round 2: box sends 24+ bytes, signalling the handshake is over. The
	// client should now send the two command packets instead of echoing.
	if _, err := box.Write(make([]byte, 24)); err != nil {
		t.Fatalf("box write command-phase trigger: %v", err)
	}

	first := make([]byte, 8)
	if _, err := io.ReadFull(box, first); err != nil {
		t.Fatalf("box read first command packet: %v", err)
	}
	second := make([]byte, 8)
	if _, err := io.ReadFull(box, second); err != nil {
		t.Fatalf("box read second command packet: %v", err)
	}

	wantFirst := []byte{4, 1, 0, 0, 0, 0, byte(224 + (code / 16)), byte(code % 16)}
	wantSecond := []byte{4, 0, 0, 0, 0, 0, byte(224 + (code / 16)), byte(code % 16)}
	if string(first) != string(wantFirst) {
		t.Errorf("first command packet = %v, want %v", first, wantFirst)
	}
	if string(second) != string(wantSecond) {
		t.Errorf("second command packet = %v, want %v (second byte must be 0)", second, wantSecond)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("sendSkyCommandOverConn returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("sendSkyCommandOverConn did not return in time")
	}
}

func TestSendSkyCommandOverConn_ReadErrorPropagates(t *testing.T) {
	client, box := net.Pipe()
	defer client.Close()

	// Close the box's end immediately so the client's first Read fails.
	box.Close()

	err := sendSkyCommandOverConn(client, 6)
	if err == nil {
		t.Fatal("expected an error when the connection closes before any data arrives, got nil")
	}
}
