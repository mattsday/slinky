# Plan: Repo resilience for agentic build / design

Status: draft — options laid out, nothing implemented yet. The backend
portions of this (Options A and B) are now sequenced into
[sky-stream-and-resilience.md](sky-stream-and-resilience.md) as Stages 0–1;
that's the doc to work from. This one still holds the fuller rationale and
the deferred Option C.

## Goal

Slinky is currently a single Go package with no tests, no CI, no linter
config, and no build tooling beyond raw `go` invocations. That's fine for a
solo passion project edited by hand, but it's a weak substrate for agentic
work (Claude Code or otherwise): there's no fast, automatic signal that a
change broke something, so every change has to be manually verified against
real hardware (a Sky box, a Harmony hub) that an agent can't reach.

This plan is about closing that gap enough that an agent (or a human) can
make a change and get a trustworthy pass/fail signal without a TV in the
loop, *before* the React rewrite and Sky Stream work land — both of those
will be much safer to execute with this in place first.

[AGENTS.md](../../AGENTS.md) (already added) covers the "how do I orient in
this repo" side of this. This plan covers the "how do I know I didn't break
it" side.

## Non-goals

- Turning this into an enterprise-grade codebase. It's a personal project;
  the bar is "an agent can verify its own work," not "100% coverage."
- Testing against real Sky Q / Harmony / Sky Stream hardware. That stays
  manual-verification-only (see the Sky Stream plan for why).
- Rewriting the frontend — that's the React plan's job, and adding a test
  harness to the current hand-rolled JS is a poor use of effort if it's
  getting replaced.

## Findings from this session (concrete, not hypothetical)

- `go build` output isn't gitignored — a stray `slinky` binary showed up as
  an untracked file the moment someone ran `go build` in the repo root.
  Cheap fix, listed below.
- `cfg.Control` is only validated at request time (`apiCall`'s `switch`
  falls through to a 500 for unknown values) — a typo in config is
  invisible until someone presses a remote button.
- Every piece of control-plane logic (`skyCommands` map, the Sky Q framing
  in `sendSkyCommand`, the Harmony URL construction, config layering/
  precedence, `hlsPlaylist` output) is pure enough to unit test today with
  zero refactoring — the blocker isn't testability, it's that no tests
  exist yet.

## Options

### A. Minimal — tests + CI for what's already testable

- `go test ./...` coverage for the pieces that need no refactor:
  - `skyCommands` map lookups / unknown-command error path (`skyq.go`)
  - `hlsPlaylist()` output format (`main.go`) via `httptest`
  - config layering precedence (base → dev → `$CONFIG_FILE` → env) using
    temp dirs and `viper` the same way `loadCfg()` does
  - `apiCall`'s dispatch on `cfg.Control` (harmony vs skyq vs unset) via
    `httptest`, with the Harmony HTTP calls hitting an `httptest.Server`
    instead of a real hub
- A GitHub Actions workflow: `go build`, `go vet`, `gofmt -l` (fail on
  output), `go test ./...`.
- A `Makefile` (or `justfile`) with `build`, `test`, `lint`, `run`,
  `docker-build` targets, so both agents and humans have one place to look
  instead of memorizing flags.
- Add `/slinky` (or `/app`, matching the Dockerfile's build output) and any
  other build artifacts to `.gitignore`.

Effort: small (a few hours). Leverage: high — this alone gives an agent a
real feedback loop for the two most change-prone files (`skyq.go`, `api.go`)
without touching production code structure.

### B. Medium — A, plus making the untestable parts testable

- Split `net.Conn` usage in `sendSkyCommand` behind a small interface (or
  accept a `net.Conn`/dialer as a parameter) so the handshake/command
  framing can be tested against an in-memory fake socket instead of a real
  Sky Q box — this is the one piece of real control-plane logic A can't
  reach.
- Add `golangci-lint` (or just `staticcheck`) to CI instead of relying on
  `go vet` alone.
- Startup-time config validation: fail fast in `loadCfg()`/`main()` if
  `cfg.Control` isn't a known value, instead of discovering it on first
  button press.
- Optional: split `main.go` into a couple of files by concern (HTTP routing
  vs. handlers vs. proxy) — no package split needed yet, just reducing the
  one 260-line file. Low priority; only worth doing if it starts getting in
  the way.

Effort: medium (adds a day or so on top of A). Leverage: closes the last
real gap (Sky Q framing) and catches config mistakes before they reach a
button press.

### C. Heavy — full service decomposition (not recommended now)

- Break `main` into `internal/config`, `internal/skyq`, `internal/harmony`,
  `internal/streaming` packages behind interfaces, full dependency
  injection, integration tests against fake TCP/WebSocket servers standing
  in for real hardware, contract tests, Dependabot/Renovate for `go.mod`,
  pre-commit hooks, mutation testing.
- This is the right shape *if* Sky Stream support (a third control backend,
  with a much heavier protocol — see that plan) lands and the `switch
  cfg.Control` pattern in `apiCall` starts feeling cramped. Doing it now,
  before that need is concrete, is speculative structure for a ~1,500-line
  Go codebase.

Effort: large. Leverage: mostly deferred value — revisit once Sky Stream
support is actually being implemented, not before.

## Recommendation

Do **A now**, do **B alongside the Sky Stream work** (since Sky Stream adds
a second network-protocol backend that benefits from the same fake-socket
testing pattern being established for Sky Q), and revisit **C only if**
`apiCall`'s three-way branch actually gets painful once Sky Stream is in.

## Rough phases

1. `.gitignore` fix, `Makefile`, GitHub Actions skeleton (build/vet/fmt).
2. Unit tests for `skyCommands`, `hlsPlaylist`, config layering.
3. `httptest`-backed tests for `apiCall`'s Harmony path and the `pwr`/
   `call/power` handlers.
4. Wire test job into CI; require it to pass.
5. (With Sky Stream work) fake-socket test harness for `sendSkyCommand`,
   startup config validation, `golangci-lint`.

## Open questions

- Is GitHub Actions the right CI target, or is this meant to stay purely
  local (no remote repo / Actions minutes to consider)?
- `Makefile` vs `justfile` vs just documenting raw commands in AGENTS.md —
  no strong signal either way yet, defaulting to `Makefile` as the more
  universally-available option unless there's a preference.
