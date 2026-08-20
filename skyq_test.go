package main

import "testing"

func TestSendSkyCommand_UnknownCommandErrors(t *testing.T) {
	err := sendSkyCommand("127.0.0.1", 1, "not-a-real-command")
	if err == nil {
		t.Fatal("expected an error for an unknown Sky Q command, got nil")
	}
}

func TestSkyCommands_ExpectedButtonsPresent(t *testing.T) {
	for _, cmd := range remoteButtonIDs {
		if _, ok := skyCommands[cmd]; !ok {
			t.Errorf("skyCommands is missing button %q, used by html/remote.html", cmd)
		}
	}
}

func TestSkyCommands_CodesAreUnique(t *testing.T) {
	// skyq.go intentionally aliases a few button names to the same code
	// (e.g. a remote might label a button "home" while the UI calls the
	// same physical button "menu"). Anything sharing a code outside this
	// known set is very likely a copy-paste mistake.
	knownAliasPairs := map[int]map[string]bool{
		2:  {"return": true, "dismiss": true},
		8:  {"interactive": true, "sidebar": true},
		10: {"search": true, "services": true},
		11: {"home": true, "menu": true},
	}

	seen := make(map[int][]string)
	for name, code := range skyCommands {
		seen[code] = append(seen[code], name)
	}
	for code, names := range seen {
		if len(names) <= 1 {
			continue
		}
		allowed := knownAliasPairs[code]
		for _, name := range names {
			if !allowed[name] {
				t.Errorf("command code %d is shared by multiple names %v, and %q is not in the known-alias allowlist", code, names, name)
			}
		}
	}
}
