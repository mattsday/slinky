package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

// This file ports the Wake-on-LAN behavior from the reference Python client
// (Soft Remote App/sky_remote.py in jatatech/sky_stream_remote): probe the
// box, send a WoL magic packet and poll for it to come back up if it's
// unreachable, then - once bound - nudge it with a Power key press, since
// WoL only wakes the box's network stack, not necessarily the TV app.

// skyStreamWoLPollInterval and skyStreamWoLMaxPolls control how long
// wakeIfNeeded waits for a box to become reachable after sending Wake-on-LAN
// before giving up and attempting to connect anyway. Matches the reference
// client's 2s x 15 (30s total). Vars (not consts) so tests can shrink the
// interval instead of taking 30 real seconds.
var (
	skyStreamWoLPollInterval = 2 * time.Second
	skyStreamWoLMaxPolls     = 15
)

// skyStreamReachable reports whether host:8091 currently accepts a TCP
// connection. A package var so tests can fake it instead of depending on
// real network state.
var skyStreamReachable = func(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 1500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// buildWoLMagicPacket builds a standard Wake-on-LAN magic packet for mac:
// 6 bytes of 0xFF followed by the 6-byte MAC address repeated 16 times
// (102 bytes total), per Docs/SKY_REMOTE_PROTOCOL.md section 2.
func buildWoLMagicPacket(mac string) ([]byte, error) {
	cleaned := strings.NewReplacer(":", "", "-", "").Replace(mac)
	macBytes, err := hex.DecodeString(cleaned)
	if err != nil || len(macBytes) != 6 {
		return nil, fmt.Errorf("invalid MAC address %q: want 6 hex-encoded octets", mac)
	}

	packet := make([]byte, 0, 6+16*len(macBytes))
	for i := 0; i < 6; i++ {
		packet = append(packet, 0xFF)
	}
	for i := 0; i < 16; i++ {
		packet = append(packet, macBytes...)
	}
	return packet, nil
}

// sendWakeOnLANTo writes a Wake-on-LAN magic packet for mac to addr over an
// already-open socket. Split from sendWakeOnLAN so tests can exercise the
// packet-sending logic against a local UDP listener instead of needing a
// real broadcast-capable network.
func sendWakeOnLANTo(conn net.PacketConn, addr net.Addr, mac string) error {
	packet, err := buildWoLMagicPacket(mac)
	if err != nil {
		return err
	}
	if _, err := conn.WriteTo(packet, addr); err != nil {
		return fmt.Errorf("failed to send WoL packet: %w", err)
	}
	return nil
}

// sendWakeOnLAN broadcasts a Wake-on-LAN magic packet for mac to UDP port
// 9, per Docs/SKY_REMOTE_PROTOCOL.md section 2. This is fire-and-forget:
// WoL has no acknowledgement, so a nil error only means the packet was
// sent, not that the box actually woke up.
func sendWakeOnLAN(mac string) error {
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return fmt.Errorf("failed to open broadcast socket: %w", err)
	}
	defer conn.Close()

	if err := enableBroadcast(conn); err != nil {
		return fmt.Errorf("failed to enable broadcast on socket: %w", err)
	}

	addr, err := net.ResolveUDPAddr("udp4", "255.255.255.255:9")
	if err != nil {
		return fmt.Errorf("failed to resolve broadcast address: %w", err)
	}
	return sendWakeOnLANTo(conn, addr, mac)
}

// enableBroadcast sets SO_BROADCAST on pc, required by the kernel before a
// UDP socket is allowed to send to a broadcast address.
func enableBroadcast(pc net.PacketConn) error {
	udpConn, ok := pc.(*net.UDPConn)
	if !ok {
		return fmt.Errorf("not a UDP connection")
	}
	rawConn, err := udpConn.SyscallConn()
	if err != nil {
		return err
	}
	var sockErr error
	if err := rawConn.Control(func(fd uintptr) {
		sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
	}); err != nil {
		return err
	}
	return sockErr
}

// wakeIfNeeded sends Wake-on-LAN and waits (briefly) for cfg.Host to become
// reachable if cfg.MAC is set and the box isn't already up. It always
// returns without error even if the box never became reachable in time -
// callers should attempt to connect regardless, same as the reference
// client does, since is_box_reachable can have false negatives.
func wakeIfNeeded(cfg SkyStream) (woke bool) {
	if cfg.MAC == "" {
		return false
	}
	addr := fmt.Sprintf("%s:8091", cfg.Host)
	if skyStreamReachable(addr) {
		return false
	}

	if err := sendWakeOnLAN(cfg.MAC); err != nil {
		// Nothing more to do - fall through and let the caller's dial
		// attempt fail with a clearer connection error.
		return true
	}

	for i := 0; i < skyStreamWoLMaxPolls; i++ {
		time.Sleep(skyStreamWoLPollInterval)
		if skyStreamReachable(addr) {
			return true
		}
	}
	return true
}
