# Backlog

Fleshed-out plans for these items live in [docs/plans/](docs/plans/).

## Workstream 1 (current priority): Sky Stream, built test-first

Repo resilience (tests, CI, tooling) and Sky Stream control support,
combined into one sequenced plan — build the test harness for what already
exists first, then implement Sky Stream against it plus new tests. See
[docs/plans/sky-stream-and-resilience.md](docs/plans/sky-stream-and-resilience.md)
for the execution order, and the two docs it draws from for full detail:
[docs/plans/agentic-resilience.md](docs/plans/agentic-resilience.md) and
[docs/plans/sky-stream-support.md](docs/plans/sky-stream-support.md)
(protocol reference for controlling a Sky Stream box locally, reverse-
engineered in <https://github.com/jatatech/sky_stream_remote>).

## Workstream 2 (deferred): Improve the UX

Rebuild the frontend from scratch in React. Deferred until Workstream 1
lands. See [docs/plans/react-rewrite.md](docs/plans/react-rewrite.md).
