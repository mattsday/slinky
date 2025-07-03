package main

import (
	"fmt"
	"log"
	"net"
	"time"
)

// skyCommands maps the remote button names to their respective command codes.
var skyCommands = map[string]int{
	"power":       0,
	"select":      1,
	"backup":      2,
	"dismiss":     2,
	"channelup":   6,
	"channeldown": 7,
	"interactive": 8,
	"sidebar":     8,
	"help":        9,
	"services":    10,
	"search":      10,
	"tvguide":     11,
	"home":        11,
	"i":           14,
	"text":        15,
	"up":          16,
	"down":        17,
	"left":        18,
	"right":       19,
	"red":         32,
	"green":       33,
	"yellow":      34,
	"blue":        35,
	"0":           48,
	"1":           49,
	"2":           50,
	"3":           51,
	"4":           52,
	"5":           53,
	"6":           54,
	"7":           55,
	"8":           56,
	"9":           57,
	"play":        64,
	"pause":       65,
	"stop":        66,
	"record":      67,
	"fastforward": 69,
	"rewind":      71,
	"boxoffice":   240,
	"sky":         241,
}

// sendSkyCommand connects to the Sky Q box and sends a single command.
func sendSkyCommand(host string, port int, command string) error {
	code, ok := skyCommands[command]
	if !ok {
		return fmt.Errorf("unknown sky command: %s", command)
	}

	// 1. Establish a TCP connection with a timeout.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to Sky Q box: %w", err)
	}
	defer conn.Close()

	// Set a deadline for the initial reads to prevent blocking indefinitely.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// 2. Perform the handshake and command sending sequence.
	// This sequence is a direct translation of the NodeJS logic.
	buffer := make([]byte, 1024)
	l := 12 // Initial length for the first part of the handshake.

	for {
		// Wait for data from the Sky Q box.
		n, err := conn.Read(buffer)
		if err != nil {
			return fmt.Errorf("failed to read from Sky Q box: %w", err)
		}

		// The server sends data in chunks. The client must respond to each chunk.
		if n < 24 {
			// Handshake phase: echo back a slice of the server's message.
			if _, err := conn.Write(buffer[:l]); err != nil {
				return fmt.Errorf("failed to write handshake data: %w", err)
			}
			// After the first echo, subsequent echoes are just 1 byte.
			l = 1
		} else {
			// Command phase: The handshake is complete, now send the command.
			commandBytes := []byte{4, 1, 0, 0, 0, 0, byte(224 + (code / 16)), byte(code % 16)}

			// Send the first command packet.
			if _, err := conn.Write(commandBytes); err != nil {
				return fmt.Errorf("failed to write command data (part 1): %w", err)
			}

			// Modify the second byte and send the second command packet.
			commandBytes[1] = 0
			if _, err := conn.Write(commandBytes); err != nil {
				return fmt.Errorf("failed to write command data (part 2): %w", err)
			}

			// The command is sent, so we are done.
			break
		}
	}

	log.Printf("Sent Sky Q command: %s", command)
	return nil
}
