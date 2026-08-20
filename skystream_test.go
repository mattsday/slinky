package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestComputeSkyStreamAuthToken_KnownAnswer checks the Go port against a
// value independently computed from the documented Python reference
// algorithm (Docs/SKY_REMOTE_PROTOCOL.md section 6) for the same inputs:
//
//	python3 -c "
//	import hashlib, base64
//	cert_fingerprint = 'a16031cd083792e156b761a5682c0069e37eeb119d7ccf32597f2f53e1fd859e'
//	pairingcode = '                 8 0'
//	controllernonce = '019ce95f-f60a-7e31-89ab-a1b2c3d4e5f6'
//	stbnonce = '6VN~E-1}]Q'
//	stage1 = bytes.fromhex(cert_fingerprint) + pairingcode.encode() + controllernonce.encode()
//	inner = hashlib.sha256(stage1).digest()
//	stage2 = stbnonce.encode() + inner + b'biT43y'
//	print(base64.b64encode(hashlib.sha256(stage2).digest()).decode())
//	"
//
// cert_fingerprint, pairingcode and stbnonce are the real values from the
// protocol doc's worked example; controllernonce is a synthetic full UUID
// since the doc's own example truncates that value with "...".
func TestComputeSkyStreamAuthToken_KnownAnswer(t *testing.T) {
	const (
		certFingerprint = "a16031cd083792e156b761a5682c0069e37eeb119d7ccf32597f2f53e1fd859e"
		pairingCode     = "                 8 0"
		controllerNonce = "019ce95f-f60a-7e31-89ab-a1b2c3d4e5f6"
		stbNonce        = "6VN~E-1}]Q"
		want            = "TU2gaNe71NOzqQwuGz8AhCNUqLW4rOJoU+lun3YPBBo="
	)

	got, err := computeSkyStreamAuthToken(certFingerprint, pairingCode, controllerNonce, stbNonce)
	if err != nil {
		t.Fatalf("computeSkyStreamAuthToken: %v", err)
	}
	if got != want {
		t.Errorf("computeSkyStreamAuthToken() = %q, want %q", got, want)
	}
}

func TestComputeSkyStreamAuthToken_DifferentInputsDifferentTokens(t *testing.T) {
	base := func() (string, error) {
		return computeSkyStreamAuthToken(
			"a16031cd083792e156b761a5682c0069e37eeb119d7ccf32597f2f53e1fd859e",
			"                 8 0",
			"019ce95f-f60a-7e31-89ab-a1b2c3d4e5f6",
			"6VN~E-1}]Q",
		)
	}
	want, err := base()
	if err != nil {
		t.Fatalf("base token: %v", err)
	}

	variants := map[string]func() (string, error){
		"pairingcode changes": func() (string, error) {
			return computeSkyStreamAuthToken(
				"a16031cd083792e156b761a5682c0069e37eeb119d7ccf32597f2f53e1fd859e",
				"                 9 0",
				"019ce95f-f60a-7e31-89ab-a1b2c3d4e5f6",
				"6VN~E-1}]Q",
			)
		},
		"controllernonce changes": func() (string, error) {
			return computeSkyStreamAuthToken(
				"a16031cd083792e156b761a5682c0069e37eeb119d7ccf32597f2f53e1fd859e",
				"                 8 0",
				"aaaaaaaa-f60a-7e31-89ab-a1b2c3d4e5f6",
				"6VN~E-1}]Q",
			)
		},
		"stbnonce changes": func() (string, error) {
			return computeSkyStreamAuthToken(
				"a16031cd083792e156b761a5682c0069e37eeb119d7ccf32597f2f53e1fd859e",
				"                 8 0",
				"019ce95f-f60a-7e31-89ab-a1b2c3d4e5f6",
				"different",
			)
		},
	}
	for name, variant := range variants {
		got, err := variant()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got == want {
			t.Errorf("%s: token unchanged (%q) - every input should affect the derived token", name, got)
		}
	}
}

func TestComputeSkyStreamAuthToken_InvalidCertFingerprint(t *testing.T) {
	_, err := computeSkyStreamAuthToken("not-hex", "code", "nonce", "stbnonce")
	if err == nil {
		t.Fatal("expected an error for a non-hex cert fingerprint, got nil")
	}
}

func TestSkyStreamPairRequest_JSONShape(t *testing.T) {
	req := skyStreamPairRequest{
		CommandName:     skyStreamCommandPairRequest,
		TID:             "test-tid",
		Name:            "Slinky",
		Manufacturer:    skyStreamManufacturer,
		Model:           skyStreamModel,
		ControllerNonce: "test-nonce",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		`"command_name":"Pair Request"`,
		`"controllernonce":"test-nonce"`,
		`"manufacturer":"Comcast"`,
		`"model":"IPRemote"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("marshaled pair request missing %q\ngot: %s", want, body)
		}
	}
}

func TestSkyStreamPairResponse_UnmarshalsProtocolExample(t *testing.T) {
	// Verbatim (whitespace aside) from Docs/SKY_REMOTE_PROTOCOL.md section 5.
	const body = `{
		"command_name": "Pair Request",
		"name": "Living Room",
		"pairingcode": "                 8 0",
		"status": true,
		"stbnonce": "6VN~E-1}]Q",
		"tid": "abc-123"
	}`

	var resp skyStreamPairResponse
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Name != "Living Room" {
		t.Errorf("Name = %q, want %q", resp.Name, "Living Room")
	}
	if resp.PairingCode != "                 8 0" {
		t.Errorf("PairingCode = %q, want the 20-char space-padded example", resp.PairingCode)
	}
	if !resp.Status {
		t.Error("Status = false, want true")
	}
	if resp.STBNonce != "6VN~E-1}]Q" {
		t.Errorf("STBNonce = %q, want %q", resp.STBNonce, "6VN~E-1}]Q")
	}
	if resp.TID != "abc-123" {
		t.Errorf("TID = %q, want %q", resp.TID, "abc-123")
	}
}

func TestSkyStreamBindResponse_UnmarshalsSuccessAndFailure(t *testing.T) {
	var success skyStreamBindResponse
	if err := json.Unmarshal([]byte(`{"command_name":"Bind Request","status":true,"bind_id":3,"tid":"abc"}`), &success); err != nil {
		t.Fatalf("Unmarshal success: %v", err)
	}
	if !success.Status || success.BindID != 3 {
		t.Errorf("success = %+v, want status=true bind_id=3", success)
	}

	var failure skyStreamBindResponse
	if err := json.Unmarshal([]byte(`{"command_name":"Bind Request","status":false,"tid":"abc"}`), &failure); err != nil {
		t.Fatalf("Unmarshal failure: %v", err)
	}
	if failure.Status {
		t.Error("failure.Status = true, want false")
	}
}

func TestSkyStreamKeyCommandRequest_JSONShape(t *testing.T) {
	req := skyStreamKeyCommandRequest{
		CommandName: skyStreamCommandKeyCommandRequest,
		TID:         "tid",
		AuthToken:   "token",
		BindID:      3,
		Cmd:         skyStreamKeyCmd,
		Key:         "Enter",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		`"command_name":"Key Command Request"`,
		`"cmd":"keyatomic"`,
		`"key":"Enter"`,
		`"bind_id":3`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("marshaled key command request missing %q\ngot: %s", want, body)
		}
	}
}

func TestSkyStreamKeyCommandResponse_UnrecognizedKeyHasError(t *testing.T) {
	var resp skyStreamKeyCommandResponse
	body := `{"command_name":"Key Command Request","status":false,"tid":"abc","error":"unrecognized"}`
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if resp.Status {
		t.Error("Status = true, want false for an unrecognized key")
	}
	if resp.Error != "unrecognized" {
		t.Errorf("Error = %q, want %q", resp.Error, "unrecognized")
	}
}

func TestSkyStreamKeys_AllRemoteButtonsMapped(t *testing.T) {
	for _, cmd := range remoteButtonIDs {
		if _, ok := skyStreamKeys[cmd]; !ok {
			t.Errorf("skyStreamKeys is missing button %q, used by html/remote.html", cmd)
		}
	}
}

func TestSkyStreamKeys_MenuIsHome(t *testing.T) {
	// Confirmed against a real box: the remote's menu button behaves as
	// "go to home screen" (Home), not the app's "more" menu (AccessMenu).
	// Regression guard - see docs/plans/sky-stream-support.md.
	if got := skyStreamKeys["menu"]; got != "Home" {
		t.Errorf(`skyStreamKeys["menu"] = %q, want "Home" (confirmed on real hardware)`, got)
	}
}

func TestSkyStreamKeys_ValuesAreNonEmptyAndUnique(t *testing.T) {
	seen := make(map[string]string) // key name -> button id
	for buttonID, key := range skyStreamKeys {
		if key == "" {
			t.Errorf("button %q maps to an empty key name", buttonID)
		}
		if other, ok := seen[key]; ok {
			t.Errorf("key %q is used by both %q and %q", key, other, buttonID)
		}
		seen[key] = buttonID
	}
}
