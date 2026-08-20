# Plan: React frontend rewrite

Status: draft — options laid out, nothing implemented yet.

## Goal

From [BACKLOG.md](../../BACKLOG.md): "Rebuild from scratch in React." From
the README's "Future plans" section: the author (not a frontend developer
when this was built) wants to drop the current hybrid — Go `html/template`
pages plus hand-rolled ES modules and Bootstrap-from-CDN — in favor of
something more maintainable, and use the opportunity to make the UI "more
beautiful" (explicitly called out as a wishlist item; today's UI is
"Bootstrap today and even then barely... works, but doesn't mean it can't be
more beautiful").

## Current state (what has to be replaced)

- Two server-rendered pages: `/` and `/video` (video + inline remote),
  `/remote` (remote-only, for pairing a controller device to a second
  screen). Templates nest — `remote.html` is embedded in both.
- Config values flow into HTML via Go template variables today
  (`{{range $i, $a := .Stream.Quality}}` etc.) — there's no JSON API for
  stream config, only for control commands and power status.
- Vanilla ES modules, no build step: [slinky.js](../../html/static/slinky.js)
  (entrypoint + keyboard shortcuts), [video.js](../../html/static/video.js)
  (hls.js/mpegts.js player, fullscreen, casting), [remote.js](../../html/static/remote.js)
  (button wiring, channel entry, mute/volume — volume/mute are handled
  entirely client-side against the `<video>` element, not sent to the
  backend), [api.js](../../html/static/api.js) (fetch wrapper for `/api/v1/*`).
- Existing JSON API (stays largely reusable — see below):
  `GET /api/v1/pwr`, `GET /api/v1/call/power`, `GET /api/v1/call/{call}`.
- `GET /playlist.m3u8` is server-generated from `stream.hls` config; actual
  video segments are *not* proxied through Go — they're fetched directly
  from wherever `stream.quality`/`stream.hls` URLs point (typically an
  nginx reverse proxy in front of the streamer box).

## Non-goals

- Changing the control-plane protocols (Sky Q / Harmony / future Sky
  Stream) — this is a frontend-only rewrite. The Go backend keeps owning
  hardware control.
- Server-side rendering or edge functions — this is a LAN app sitting
  behind the user's own reverse proxy + SSO; there's no SEO or
  cold-start-latency case for SSR here.

## Options

### 1. Framework choice: React + Vite (SPA) vs. Next.js

The README specifically says "React + Next.js." But nothing about this app
benefits from what Next.js adds over a plain Vite SPA:

- No public pages that need SEO.
- No serverless/edge deployment target — this always runs as a long-lived
  container next to (or as) the Go binary, because the Go binary is the
  thing holding the TCP connection to the Sky box / talking to Harmony.
  Next.js's API routes would just be a second backend to keep in sync with
  the Go one, for no benefit.
- Next.js's routing/SSR machinery is overhead for what's functionally a
  two-screen app (video+remote, remote-only).

**Recommendation: plain React + Vite SPA.** It gets the maintainability and
component-model benefits the backlog item is actually after, without taking
on a framework whose signature features (SSR, API routes, edge deploy)
don't apply here. Flagging this as a deliberate deviation from the README's
wording rather than silently picking one — worth confirming before building.

### 2. Migration strategy: big-bang vs. incremental (strangler)

- **Big-bang**: build the new app in a `web/` directory against the
  existing JSON API (plus one or two new endpoints, see below), get it to
  parity, cut the Go server over to serving the built static bundle, delete
  `html/`.
- **Incremental**: mount React "islands" into the existing templates one
  piece at a time (e.g., remote panel first, video player second).

**Recommendation: big-bang.** The entire current frontend is ~900 lines
across HTML/JS. An incremental/island approach earns its complexity on
much larger apps; here it would mean running two build/rendering systems
side by side for longer than the rewrite itself takes.

### 3. Styling

Current: Bootstrap 5 + Font Awesome from CDN, minimal custom CSS
(`slinky.css`). Options for the rewrite:

- Keep Bootstrap (via `react-bootstrap` or just the CSS + own markup) —
  fastest path to parity, doesn't address the "more beautiful" wishlist.
  Also stops loading Bootstrap's *JS* bundle for no reason — nothing in
  the current app uses Bootstrap's interactive components, only its grid
  and button classes.
- Move to Tailwind + shadcn/ui — bigger lift, but directly addresses the
  "more beautiful UI" wishlist and there's already a `shadcn-ui` skill
  available in this environment to lean on for component work.

**Recommendation:** worth spending real design time here since "more
beautiful" is explicitly the point, not just a side effect — but this is
a genuine open choice, not a foregone one; see open questions.

## API changes needed

Mostly additive — the control API doesn't need to change shape:

- **New**: `GET /api/v1/streams` (or similar) returning `cfg.Stream` as
  JSON (quality list + HLS list), so the SPA can build the quality
  selector without server-side templating. Today this only exists baked
  into the `<select>` options in `video.html`.
- **Unchanged**: `GET /api/v1/pwr`, `GET /api/v1/call/{call}`,
  `GET /playlist.m3u8`.
- **Removed** (once cutover completes): `home`/`video`/`remote` Go
  handlers and `html/template` usage — replaced by serving the Vite build
  output as static files (Go keeps the dev-mode CORS proxy for `.ts`/
  `.m3u8`/`.flv`, since that's independent of how the shell page is
  rendered).

## Component breakdown (parity target)

- `VideoPlayer` — wraps hls.js/mpegts.js source switching, `localStorage`
  quality persistence, click-to-pause.
- `QualitySelector` — fed by the new `/api/v1/streams` endpoint.
- `FullscreenOverlay` — fullscreen toggle, auto-hide-on-idle controls
  (currently ~60 lines of manual class toggling in `video.js`).
- `RemoteControl` — button grid, channel number entry with the existing
  400ms-between-digits pacing, keyboard shortcuts (currently global
  `keydown` handling in `slinky.js`).
- `PowerStatus` — polls `/api/v1/pwr` every 10s, drives the power button's
  visual state and enables/disables the rest of the remote (parity with
  today's `updateStatus()`/`toggleRemoteEnabled()`).
- `CastButton` — Remote Playback API wrapper, hidden when unavailable.

## Testing

This is also the frontend's first-ever test coverage:

- Component tests (Vitest + Testing Library) for `RemoteControl` (button →
  API call), `QualitySelector` (persistence + change events), `PowerStatus`
  (polling/enable-disable logic) — these are the pieces with real branching
  logic worth protecting.
- Skip testing `VideoPlayer`'s actual playback (hls.js/mpegts.js internals
  aren't meaningfully unit-testable) — smoke-test manually per the golden
  path in AGENTS.md/CLAUDE guidance instead.

## Phased plan

0. Decide framework (§1) and styling (§3) — see open questions.
1. Scaffold `web/` (Vite + React + TS), wire up dev proxy to the Go API for
   local development.
2. Add `GET /api/v1/streams` to the Go backend.
3. Build `VideoPlayer` + `QualitySelector` to parity.
4. Build `RemoteControl` + `PowerStatus` + keyboard shortcuts to parity.
5. Styling/polish pass (the actual "more beautiful UI" work).
6. Wire Go to serve the built bundle; update `Dockerfile` with a Node build
   stage; update `samples/` and README's setup instructions.
7. Delete `html/`, cut over, remove the now-dead Go template handlers.

## Open questions

- Confirm plain React + Vite is acceptable given the README says
  "Next.js" — or was that just the author's shorthand for "modern React
  setup" rather than a real requirement?
- Bootstrap-parity-first vs. Tailwind/shadcn redesign — affects how much
  of phase 5 is "polish" vs. "from-scratch design work," worth deciding
  before phase 0 rather than mid-rewrite.
- Does `/remote`-only (second-screen companion) stay a separate route in
  the SPA, or was that a workaround for template duplication that a
  component-based app no longer needs as a distinct page?
