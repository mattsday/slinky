# Plan: Sky Stream support, built test-first (Workstream 1)

Status: draft — the ordered execution plan for the current priority.
Combines [agentic-resilience.md](agentic-resilience.md) and
[sky-stream-support.md](sky-stream-support.md) into one sequenced backlog;
those docs still hold the fuller options/rationale, this one is what
actually gets worked. [react-rewrite.md](react-rewrite.md) (Workstream 2)
is deferred until this lands.

## Why combined

Backend tests and Sky Stream aren't two projects — they're one, done in the
right order: build tests for what already exists so there's a measurable
"doneness" baseline, then implement Sky Stream against that harness plus
new Sky-Stream-specific tests. Sky Stream is also the first backend feature
that genuinely needs the fake-socket/fake-server testing pattern (it's a
long-lived, stateful, reconnecting session — nothing existing today is),
so it's a natural forcing function for the resilience work rather than a
reason to defer it.

## Definition of done (the "doneness" bar)

Concrete, checkable gates rather than vague "add tests":

- `go build`, `go vet`, `gofmt -l` clean, `go test ./...` green, wired into
  CI, before any Sky Stream code lands.
- Existing control-plane logic has tests: `skyCommands` map + unknown-
  command error path, `hlsPlaylist` output, config layering precedence,
  `apiCall`'s harmony dispatch (via `httptest`) and skyq dispatch (via a
  fake socket, see Stage 1).
- New Sky Stream code lands with tests for: token derivation (known-answer
  from the protocol doc's worked example), JSON message (de)serialization
  for each protocol step, the command-name mapping table, and the
  pair→bind→key-command state machine exercised against a local fake mTLS
  WebSocket server — not left as "manual only" (see the note below on why
  this raises the bar from the original Sky Stream plan).
- Only the final "does a real Sky Stream box actually accept this and do
  the right thing" check remains manual — everything else in the protocol
  client is asserted by tests before that point.

## Sequencing

### Stage 0 — Baseline harness

*(agentic-resilience.md, Option A)*

1. `.gitignore` fix for build artifacts (the stray `slinky` binary found
   during the repo audit).
2. `Makefile` with build/test/lint/run targets.
3. GitHub Actions: build, vet, `gofmt -l` check, test.
4. Tests: `skyCommands` map, `hlsPlaylist`, config layering precedence
   (base → dev → `$CONFIG_FILE` → env), `apiCall`'s harmony dispatch via
   `httptest`.

**Exit criterion**: CI is green on a no-op change. This is the baseline
the rest of the plan measures "doneness" against.

### Stage 1 — Fake-socket pattern, applied to existing Sky Q code

*(agentic-resilience.md, Option B, narrowed to just what Sky Stream needs)*

5. Abstract the `net.Conn` usage in `sendSkyCommand` behind a dialer/
   interface so the handshake+command framing is testable against an
   in-memory fake socket instead of a real Sky Q box.
6. Startup-time config validation for `cfg.Control` (fail fast on an
   unknown value instead of discovering it on first button press) — small,
   but establishes that config correctness is asserted at startup, a
   pattern Sky Stream's new config keys should also follow.

**Exit criterion**: Sky Q's TCP framing has the same test coverage
`skyCommands` already has. This stage exists to produce the fake-socket/
fake-server pattern Sky Stream reuses next, not because Sky Q needs more
coverage for its own sake.

### Stage 2 — Sky Stream: pure, testable pieces first

*(sky-stream-support.md phases 1–2, reordered test-first)*

7. Confirm licensing for vendoring the reference repo's embedded mTLS
   cert/key — blocks everything downstream, do this before writing code
   that depends on it.
8. Port token derivation (`compute_authtoken`) with a known-answer test
   from the protocol doc's worked example.
9. Define the JSON message structs (Pair/Bind/Key Command request +
   response) with serialize/deserialize round-trip tests.
10. Define the command-name mapping table (Slinky button ID → Sky Stream
    key name, per the table in sky-stream-support.md) with a test asserting
    every button ID currently in `remote.html` has a mapping.

**Exit criterion**: everything in Sky Stream that doesn't require a live
network connection is under test.

### Stage 3 — Sky Stream: session/transport, tested against a fake server

11. Build a local fake Sky STB test double: a TLS test server requiring a
    client certificate (mirrors the real box's mTLS requirement; server-
    side cert verification can stay disabled in the test double the same
    way the reference implementation disables it on the client side) that
    upgrades to `/iptarget` and speaks the pair/bind/key-command JSON
    protocol.
12. Implement the real client (TLS+mTLS dial, WebSocket upgrade,
    pair → derive token → bind → send key command) against that fake
    server, under test.
13. Wire into `apiCall` as `control: skystream`; add config plumbing
    (`sky_stream.host`, optional `mac`).

**Exit criterion**: the full session state machine is exercised by
`go test ./...`, with no real hardware involved.

**Done.** `skystream.go` (pure protocol pieces) and `skystream_transport.go`
(dial, session, `apiCall` wiring) landed, tested via a fake box test double
(`skystream_transport_test.go`) that speaks real TLS with mandatory mTLS
and a real WebSocket upgrade — including a test that independently
re-derives the expected auth token server-side from the client cert
actually presented over the connection, and a test confirming the fake
box genuinely rejects connections without a client cert (so the "success"
tests are proving something). `go test ./... -race` is clean; both the
Makefile and CI now run tests with `-race` given the session code's shared
mutex-protected state. Total: 38 tests.

### Stage 4 — Real-hardware verification (necessarily manual)

14. Point it at a real Sky Stream box. Verify pairing doesn't trigger the
    documented lockout behavior, and resolve the two open mapping
    questions from sky-stream-support.md (`return` → `Backspace` vs.
    `Dismiss`; `menu` → `AccessMenu` vs. `Home`).
15. README config docs (new `control: skystream` section, matching the
    existing Sky Q / Harmony tables).

**Exit criterion**: this is the only stage that can't be automated —
everything upstream of it is already verified by CI.

**Done (mostly).** Verified end-to-end against two real boxes on the LAN:

- `.106` (WiFi, already associated): dial → pair → bind succeeded first
  real attempt after two transport fixes the fake-server tests couldn't
  have caught (see below).
- `.122` (ethernet, asleep): full chain including Wake-on-LAN — our own
  `sendWakeOnLAN` woke it, `connectSkyStream` polled it back to reachable,
  paired, bound, sent the automatic post-wake `Power` nudge, and then a
  real `Info` key command was sent and accepted.

Two real bugs found and fixed, neither visible to the fake-server tests
because `gorilla/websocket`'s TLS wrapping behaved identically against
both the fakes and the real box — the *box's* TLS/HTTP stack was pickier
than Go's or our fakes' defaults:

1. **TLS config was missing ALPN and a forced TLS 1.3 floor.** Without
   `NextProtos: []string{"http/1.1"}` and `MinVersion: tls.VersionTLS13`,
   the handshake failed with a bare `unexpected EOF` - confirmed via
   `openssl s_client` succeeding with identical cert/SNI/TLS1.3 while our
   Go dial failed, isolating it to config rather than code structure.
2. **The WebSocket upgrade request had no headers.** The box's embedded
   HTTP server silently closed the connection without `Origin`,
   `User-Agent: Dart/3.9 (dart:io)`, `Cache-Control: no-cache`, and
   `Accept-Encoding: gzip` set - all documented in
   `Docs/SKY_REMOTE_PROTOCOL.md` section 4 but not encoded in `dialSkyStream`
   until this point.

**Wake-on-LAN** (`skystream_wol.go`) was implemented in this session,
ported from the reference Python client's wake→poll→connect→nudge pattern,
and works end-to-end for ethernet. It also surfaced an environment lesson
worth recording: a UDP broadcast to `255.255.255.255` only reaches the
sender's own L2 segment - it does **not** cross routed/NAT boundaries. Sent
from this project's sandboxed dev container (on its own isolated subnet),
WoL silently went nowhere; sent from a `--network host` sibling container
(sharing the actual host machine's network namespace, which sits directly
on the target LAN), it worked. Two WiFi boxes (`.137`, `.193`) never woke
even with genuine L2 broadcast access - the host's ARP table showed
`FAILED` (no L2 response at all) for both, pointing at the access point
not forwarding wake frames to fully-disassociated WiFi clients, a common
router-side WoWLAN limitation rather than a bug here. Treated as an
accepted, out-of-scope limitation for now rather than something to keep
chasing from this codebase.

**Still open**: the two key-name ambiguities (`return` → `Backspace` vs.
`Dismiss`; `menu` → `AccessMenu` vs. `Home`) haven't specifically been
exercised yet - only `Info` has been real-hardware-verified so far.

**Real-deployment regression found and fixed**: deploying `config-stream.yaml`
to the actual production docker-compose (bridge-networked, not the
`--network host` sibling container used for testing in this session) hit
exactly the WoL-broadcast-can't-cross-a-bridge-network issue predicted
above: `.122` was asleep, `sendWakeOnLAN` fired but never reached the LAN,
and the subsequent dial failed with a bare `no route to host` giving no
clue why. Two fixes landed from this:

- `samples/docker-compose.yaml` and the README's Sky Stream config section
  now both document that `network_mode: host` is required whenever
  `sky_stream.mac` is set, matching the precedent already set by
  `samples/harmony-api/docker-compose.yaml`.
- `connectSkyStream`'s error now says so directly: if a dial fails after a
  WoL attempt, the error is wrapped with a hint pointing at
  `network_mode: host`, tested in
  `TestConnectSkyStream_FailedDialAfterWoLHintsAtHostNetworking` (and
  `..._WithoutWoLHasNoHint` confirms it stays out of unrelated errors).
  Total: 52 tests.

## Raising the bar on Sky Stream testability

The original [sky-stream-support.md](sky-stream-support.md) treated the
whole TLS+WebSocket+pair+bind session as "manual-verification-only, same
as Sky Q." That's overly conservative on reflection: Sky Q's protocol is a
raw TCP byte exchange with real-box-specific handshake quirks, genuinely
hard to fake convincingly. Sky Stream's protocol is JSON over WebSocket
over TLS — a boring, well-specified transport that Go's standard test
tooling (`httptest`, `crypto/tls` with a self-signed test cert pair) can
fake faithfully. Stage 3 builds that fake server so the *protocol logic*
gets real CI coverage; only hardware-specific behavior (does the real box
lock out, does `AccessMenu` really open the menu on this remote) stays
manual.

## What this deliberately leaves out

Frontend replacement ([react-rewrite.md](react-rewrite.md), Workstream 2)
is deprioritized behind this per current direction. Nothing here blocks
it — Sky Stream only touches the Go control-plane backend — it's simply
not scheduled until this workstream lands.
