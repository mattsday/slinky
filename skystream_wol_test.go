package main

import (
	"encoding/hex"
	"net"
	"strings"
	"testing"
	"time"
)

func TestBuildWoLMagicPacket_KnownAnswer(t *testing.T) {
	// The .122 box's real MAC from this session's manual hardware testing.
	got, err := buildWoLMagicPacket("04:b8:6a:f2:e7:4c")
	if err != nil {
		t.Fatalf("buildWoLMagicPacket: %v", err)
	}
	if len(got) != 102 {
		t.Fatalf("packet length = %d, want 102 (6 sync bytes + 16x6-byte MAC)", len(got))
	}
	for i := 0; i < 6; i++ {
		if got[i] != 0xFF {
			t.Errorf("byte %d = %#x, want 0xff (sync stream)", i, got[i])
		}
	}
	macBytes, _ := hex.DecodeString("04b86af2e74c")
	for rep := 0; rep < 16; rep++ {
		start := 6 + rep*6
		if string(got[start:start+6]) != string(macBytes) {
			t.Errorf("MAC repetition %d = %x, want %x", rep, got[start:start+6], macBytes)
		}
	}
}

func TestBuildWoLMagicPacket_AcceptsHyphenatedMAC(t *testing.T) {
	colon, err := buildWoLMagicPacket("04:b8:6a:f2:e7:4c")
	if err != nil {
		t.Fatalf("colon form: %v", err)
	}
	hyphen, err := buildWoLMagicPacket("04-b8-6a-f2-e7-4c")
	if err != nil {
		t.Fatalf("hyphen form: %v", err)
	}
	if string(colon) != string(hyphen) {
		t.Error("colon-separated and hyphen-separated MACs produced different packets")
	}
}

func TestBuildWoLMagicPacket_InvalidMAC(t *testing.T) {
	for _, bad := range []string{"", "not-a-mac", "04:b8:6a:f2:e7", "04:b8:6a:f2:e7:4c:00"} {
		if _, err := buildWoLMagicPacket(bad); err == nil {
			t.Errorf("buildWoLMagicPacket(%q): expected an error, got nil", bad)
		}
	}
}

func TestSendWakeOnLANTo_SendsCorrectPacket(t *testing.T) {
	listener, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket: %v", err)
	}
	defer listener.Close()

	sender, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket (sender): %v", err)
	}
	defer sender.Close()

	const mac = "04:b8:6a:f2:e7:4c"
	if err := sendWakeOnLANTo(sender, listener.LocalAddr(), mac); err != nil {
		t.Fatalf("sendWakeOnLANTo: %v", err)
	}

	listener.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 256)
	n, _, err := listener.ReadFrom(buf)
	if err != nil {
		t.Fatalf("ReadFrom: %v", err)
	}

	want, err := buildWoLMagicPacket(mac)
	if err != nil {
		t.Fatalf("buildWoLMagicPacket: %v", err)
	}
	if string(buf[:n]) != string(want) {
		t.Errorf("received packet = %x, want %x", buf[:n], want)
	}
}

func TestWakeIfNeeded_NoMACConfiguredSkipsEverything(t *testing.T) {
	calls := 0
	orig := skyStreamReachable
	skyStreamReachable = func(addr string) bool { calls++; return false }
	defer func() { skyStreamReachable = orig }()

	woke := wakeIfNeeded(SkyStream{Host: "10.0.0.1"})
	if woke {
		t.Error("wakeIfNeeded returned true with no MAC configured")
	}
	if calls != 0 {
		t.Errorf("skyStreamReachable called %d times, want 0 (nothing to wake for)", calls)
	}
}

func TestWakeIfNeeded_AlreadyReachableSkipsWoL(t *testing.T) {
	origReachable := skyStreamReachable
	skyStreamReachable = func(addr string) bool { return true }
	defer func() { skyStreamReachable = origReachable }()

	woke := wakeIfNeeded(SkyStream{Host: "10.0.0.1", MAC: "04:b8:6a:f2:e7:4c"})
	if woke {
		t.Error("wakeIfNeeded returned true when the box was already reachable")
	}
}

func TestWakeIfNeeded_UnreachableThenBecomesReachable(t *testing.T) {
	origReachable := skyStreamReachable
	origInterval := skyStreamWoLPollInterval
	skyStreamWoLPollInterval = time.Millisecond
	defer func() {
		skyStreamReachable = origReachable
		skyStreamWoLPollInterval = origInterval
	}()

	calls := 0
	skyStreamReachable = func(addr string) bool {
		calls++
		return calls > 3 // becomes reachable on the 4th check (1 initial + 3 polls)
	}

	woke := wakeIfNeeded(SkyStream{Host: "127.0.0.1", MAC: "04:b8:6a:f2:e7:4c"})
	if !woke {
		t.Error("wakeIfNeeded returned false, want true (WoL was needed)")
	}
	if calls != 4 {
		t.Errorf("skyStreamReachable called %d times, want 4 (stop polling once reachable)", calls)
	}
}

func TestWakeIfNeeded_GivesUpAfterMaxPolls(t *testing.T) {
	origReachable := skyStreamReachable
	origInterval := skyStreamWoLPollInterval
	origMaxPolls := skyStreamWoLMaxPolls
	skyStreamWoLPollInterval = time.Millisecond
	skyStreamWoLMaxPolls = 3
	defer func() {
		skyStreamReachable = origReachable
		skyStreamWoLPollInterval = origInterval
		skyStreamWoLMaxPolls = origMaxPolls
	}()

	calls := 0
	skyStreamReachable = func(addr string) bool { calls++; return false }

	woke := wakeIfNeeded(SkyStream{Host: "127.0.0.1", MAC: "04:b8:6a:f2:e7:4c"})
	if !woke {
		t.Error("wakeIfNeeded returned false, want true (a WoL attempt was made even though it never became reachable)")
	}
	// 1 initial reachability check + skyStreamWoLMaxPolls polls.
	if calls != 1+skyStreamWoLMaxPolls {
		t.Errorf("skyStreamReachable called %d times, want %d", calls, 1+skyStreamWoLMaxPolls)
	}
}

func TestWakeIfNeeded_InvalidMACDoesNotPanic(t *testing.T) {
	origReachable := skyStreamReachable
	skyStreamReachable = func(addr string) bool { return false }
	defer func() { skyStreamReachable = origReachable }()

	// buildWoLMagicPacket will fail inside sendWakeOnLAN; wakeIfNeeded
	// should still return cleanly rather than panicking, letting the
	// caller's connection attempt fail with a clearer error instead.
	woke := wakeIfNeeded(SkyStream{Host: "127.0.0.1", MAC: "not-a-mac"})
	if !woke {
		t.Error("wakeIfNeeded returned false for a configured (if invalid) MAC")
	}
}

func TestEnableBroadcast_RejectsNonUDPConn(t *testing.T) {
	// enableBroadcast requires a *net.UDPConn specifically (to reach its
	// SyscallConn); a conn of any other concrete type must be rejected
	// rather than type-asserted blindly.
	if err := enableBroadcast(fakePacketConn{}); err == nil {
		t.Fatal("expected an error for a non-UDP PacketConn, got nil")
	}
}

// fakePacketConn is a minimal net.PacketConn that isn't a *net.UDPConn, to
// exercise enableBroadcast's type-assertion error path.
type fakePacketConn struct{ net.PacketConn }

func TestWoLIntegration_ButtonMappingHasPower(t *testing.T) {
	// connectSkyStream's post-WoL nudge sends skyStreamKeys["power"] -
	// guard against that entry ever being removed from the mapping table.
	if _, ok := skyStreamKeys["power"]; !ok {
		t.Fatal(`skyStreamKeys is missing "power" - connectSkyStream's post-WoL nudge depends on it`)
	}
}

func TestBuildWoLMagicPacket_CaseInsensitive(t *testing.T) {
	lower, err := buildWoLMagicPacket("04:b8:6a:f2:e7:4c")
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	upper, err := buildWoLMagicPacket(strings.ToUpper("04:b8:6a:f2:e7:4c"))
	if err != nil {
		t.Fatalf("upper: %v", err)
	}
	if string(lower) != string(upper) {
		t.Error("MAC case affected the resulting packet")
	}
}
