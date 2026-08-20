# Plan: Sky Stream control support

Status: draft — protocol researched and documented below, nothing
implemented yet. The implementation is now sequenced (test-first, alongside
the resilience work) in
[sky-stream-and-resilience.md](sky-stream-and-resilience.md) — that's the
doc to work from. This one holds the full protocol reference and rationale.

## Goal

From [BACKLOG.md](../../BACKLOG.md): control a Sky Stream / Sky Glass box
over the local network, as a third `control` backend alongside the existing
`skyq` (raw TCP to a Sky Q box) and `harmony` (REST calls to a Logitech
Harmony Hub). Sky is moving customers from Sky Q to Sky Stream/Sky Glass
hardware, which does not speak Sky Q's protocol — this is a different box
needing a different implementation, not a config tweak to `skyq.go`.

The backlog pointed at `github.com/jatatech/sky_stream_remote` (Python) as
a reference. That repo turned out to include full reverse-engineering
documentation of the protocol (`Docs/SKY_REMOTE_PROTOCOL.md`, reversed from
the official "Sky Remote" Android app, APK v1.0.7291), which is summarized
below so the Go port doesn't need to re-derive it.

## Protocol summary (from the reference repo's research)

This is a materially different — and heavier — protocol than Sky Q's raw
TCP command bytes:

1. **Discovery**: mDNS, service type `_rdk-rics._tcp.local.`, advertises
   port `8091` and a `wol_mac` property for Wake-on-LAN.
2. **Wake-on-LAN** (if the box is in standby): standard magic packet (6×
   `0xFF` + MAC ×16) to `255.255.255.255:9`.
3. **Transport**: TCP port `8091`, **TLS 1.3 with mandatory mutual TLS** —
   the box rejects connections without a client certificate. Server
   certificate verification is disabled (`CERT_NONE`) in the reference
   implementation; SNI is `sky.xcal.tv`.
4. **Client identity**: an EC P-256 certificate chain + private key that is
   *embedded in the official app and shared across every install* — not
   per-user, not per-device. The reference repo vendors these (extracted
   from the APK) under `Soft Remote App/certs/`.
5. **WebSocket upgrade** at `wss://<box-ip>:8091/iptarget` over the TLS
   connection.
6. **Session**: JSON messages over the WebSocket —
   `Pair Request` (client sends a random `controllernonce`) →
   `Pair Response` (box returns `pairingcode` + `stbnonce`) →
   client computes an `authtoken` (two-stage SHA-256: `SHA256(cert_fingerprint
   ‖ pairingcode ‖ controllernonce)`, then `SHA256(stbnonce ‖ that ‖
   "biT43y")`, base64-encoded — `"biT43y"` is a hardcoded salt from the
   app binary) →
   `Bind Request` with that token → `Bind Response` returns a `bind_id` →
   `Key Command Request` (`cmd: "keyatomic"`, `key: "<KeyName>"`, plus the
   `authtoken`/`bind_id`) for every button press.
7. **Key names** are a fixed vocabulary (`ArrowUp`, `Enter`, `Power`,
   `Digit0`–`Digit9`, `ChannelUp`, `VolumeUp`, `MediaPlay`, `Red`, etc.) —
   not the same strings Sky Q uses, and not W3C standard key names either.
8. The reference doc explicitly warns: **repeated failed bind attempts can
   lock the box out until it's rebooted** — this affects both the
   implementation approach (get it right in a low-risk test harness before
   hammering a real box) and the retry/backoff policy in the code.
9. **Not in the reference doc, found during real-hardware verification**:
   the box's TLS stack requires ALPN `http/1.1` and rejects anything below
   TLS 1.3 with a bare connection close (no alert) rather than a clean
   error - and its embedded WebSocket server silently drops the upgrade
   request unless `Origin`, `User-Agent: Dart/3.9 (dart:io)`,
   `Cache-Control: no-cache` and `Accept-Encoding: gzip` are all present,
   even though the reference doc lists these as descriptive ("what the app
   sends") rather than flagging them as required. Both are now set
   explicitly in `dialSkyStream` (skystream_transport.go).

## Command-name mapping

Slinky's existing button vocabulary (from `remote.html`'s button IDs,
already shared today between the frontend and both `skyq`/`harmony`
backends) maps fairly cleanly onto the Sky Stream key names:

| Slinky button ID | Sky Stream key      | Notes |
|-------------------|----------------------|-------|
| `power`            | `Power`              | |
| `select`           | `Enter`              | |
| `return`           | `Backspace`          | Sky Stream also has a separate `Dismiss` key; Sky Q aliases `return`/`dismiss` to the same code, Sky Stream doesn't — needs a real-box check for which reads better as "back". |
| `channel-up`       | `ChannelUp`          | |
| `channel-down`     | `ChannelDown`        | |
| `info`             | `Info`               | |
| `search`           | `Search`             | |
| `menu`             | `Home`               | **Confirmed on real hardware**: the remote's menu button behaves as "go to home screen", not the app's "more" menu (`AccessMenu`). |
| `direction-up/down/left/right` | `ArrowUp`/`ArrowDown`/`ArrowLeft`/`ArrowRight` | |
| `red`/`green`/`yellow`/`blue`  | `Red`/`Green`/`Yellow`/`Blue` | |
| `0`–`9`             | `Digit0`–`Digit9`    | |
| `play`              | `MediaPlay`          | Play/pause toggle, same as today's single "play" button. |
| `rewind`            | `MediaRewind`        | |
| `fast-forward`      | `MediaFastForward`   | |
| `record`            | `MediaRecord`        | |

`volume-up`/`volume-down`/`mute` stay client-side against the `<video>`
element as they are today (never sent to a control backend) — though Sky
Stream does expose real `VolumeUp`/`VolumeDown`/`VolumeMute` keys, which
would let a future iteration control the box's actual output volume
instead of just the browser tab's — noted as a follow-up, not v1 scope.

## Design in Go

**Implemented** as two files, mirroring the split `skyq.go`/Stage 1 of the
resilience work established: `skystream.go` holds the pure protocol pieces
(message structs, auth token derivation, command mapping table — all unit
tested with no network involved), `skystream_transport.go` holds the
TLS+mTLS+WebSocket dial, the pair/bind/send-key session, and the
`apiCall` wiring (tested against a fake box double, see
sky-stream-and-resilience.md Stage 3). Selected via `control: skystream`,
a third case in `apiCall`'s `switch cfg.Control`.

New dependencies needed (none of these exist in `go.mod` today):

- A WebSocket client — `nhooyr.io/websocket` or `github.com/gorilla/websocket`.
- An mDNS client for discovery — e.g. `github.com/grandcat/zeroconf` —
  *if* discovery is implemented (see phasing below; can be deferred in
  favor of a configured host, same as `sky_q.host` today).
- TLS + the two-stage SHA-256 token derivation need nothing beyond the
  standard library (`crypto/tls`, `crypto/sha256`, `encoding/base64`).

**Implemented** in [config.go](../../config.go), mirroring the existing
`SkyQ` struct:

```go
type SkyStream struct {
    Host     string `mapstructure:"host"`
    MAC      string `mapstructure:"mac"`       // optional; enables Wake-on-LAN
    CertFile string `mapstructure:"cert_file"`
    KeyFile  string `mapstructure:"key_file"`
}
```

Session handling: pair + bind once per connection (lazily, on first
command, or eagerly at startup — open question below), hold the `bind_id`/
`authtoken` for the process lifetime, reconnect-and-rebind on WebSocket
drop. This is a meaningfully more stateful backend than `skyq.go` (which
opens a fresh TCP connection per command) or `api.go`'s Harmony client
(stateless HTTP) — it's the first backend in this codebase that needs a
long-lived, reconnecting connection, which is worth keeping in mind for the
resilience plan's "when do we need package decomposition" question.

## Vendoring the embedded credentials

The mTLS client cert/key are load-bearing for this protocol and are shared
across every install of the official app (extracted from the APK, not
generated per-device). Before porting:

- **Resolved**: `jatatech/sky_stream_remote` carries no license file at
  all, so nothing is copied from it verbatim. The protocol logic (message
  shapes, token derivation, key names) is independently implemented from
  the documented algorithm — the same relationship `skyq.go` already has
  to `github.com/dalhundal/sky-remote` — with attribution in `skystream.go`.
  The mTLS cert/key themselves are **not vendored into this repo**; Sky
  Stream config takes cert/key file *paths*, supplied locally by whoever
  runs Slinky, the same way secrets normally shouldn't be committed
  regardless of licensing.
- These are shared/static secrets Sky could rotate or revoke at any time,
  same risk class as the existing Sky Q integration relying on an
  undocumented protocol. Not a blocker, just worth stating plainly: this
  backend can break if Sky changes the app's embedded credentials, with no
  advance notice.

## Testing

No CI-reachable hardware exists for this (same constraint as Sky Q). What
*is* testable without a real box, per the resilience plan's fake-socket
pattern:

- Token derivation (`compute_authtoken` port) — pure function, exact
  known-answer test achievable from the reference doc's worked example.
- JSON message (de)serialization for each step (Pair/Bind/Key Command
  request+response).
- Command-name mapping table (§ above).

The TLS+WebSocket+pair+bind session flow itself stays manual-verification-
only against a real Sky Stream box, same as Sky Q today.

## Phased plan

1. Confirm licensing for the vendored cert/key (blocks everything else).
2. Port token derivation + message structs, with known-answer unit tests.
3. Implement the TLS+mTLS+WebSocket session (connect, pair, bind) against a
   real box — manual verification, no shortcuts here.
4. Implement `Key Command Request` sending + the command-name mapping
   table; wire into `apiCall` as `control: skystream`.
5. Config plumbing (`sky_stream.host`, optional `mac`), README docs.
6. Follow-up (not v1): mDNS discovery instead of a configured host
   (**Wake-on-LAN is done** — see below), routing `volume-up`/
   `volume-down`/`mute` to the box's real `VolumeUp`/`VolumeDown`/
   `VolumeMute` keys instead of client-side-only.

**Wake-on-LAN: implemented** (`skystream_wol.go`), ported from the
reference Python client's wake→poll→connect→post-wake-`Power`-nudge
pattern, and verified end-to-end against a real ethernet-connected box.
Verified *not* to work against two WiFi boxes that showed no ARP presence
at all (`ip neigh` reported `FAILED`) — an access-point/WoWLAN limitation,
not a bug here; see sky-stream-and-resilience.md Stage 4 for the full
writeup. One environment-level gotcha worth remembering for any future
network code in this repo: a UDP broadcast to `255.255.255.255` only
reaches the sender's own L2 segment and does not cross routed/NAT
boundaries, so WoL (and anything else broadcast-based) needs to run
somewhere with genuine L2 presence on the target network, not just routed
reachability to individual hosts.

## Open questions

- **Resolved**: pair/bind happens lazily, on first command
  (`sendSkyStreamCommand` in skystream_transport.go caches the session and
  reconnects once on failure) — startup always succeeds even if the box is
  unreachable at boot.
- **Resolved**: `menu` → `Home`, confirmed on real hardware (matches the
  physical remote's menu button behavior).
- `return` → `Backspace` vs. `Dismiss` — still open.
- Does this replace `skyq` entirely once Sky rolls out Sky Stream broadly,
  or do both stay supported side by side (i.e. is `skyq.go` here to stay)?
  Affects whether `apiCall`'s growing `switch` is a sign to do the "C"
  option from the resilience plan sooner.
