# AGENTS.md

Guidance for agents working in this repository.

## Commands

```sh
go build -o app .        # build the server binary
go run .                 # run locally (reads config/config.yaml)
go vet ./...             # static checks
gofmt -l .               # list files needing formatting (gofmt -w . to fix)
go test ./...            # no test files exist yet — this is a no-op
```

There is no test suite, linter config, or CI pipeline in this repo currently.

Local run needs `config/config.yaml` to exist (see `config/config.yaml` for the
full example) since `loadCfg()` in [main.go](main.go) fails fast if it's missing.

Docker build: `docker build .` (see [Dockerfile](Dockerfile) — multi-stage,
copies `config/` and `html/` alongside the compiled binary into a distroless
image).

## Architecture

Slinky is a single-binary Go server (package `main`, no internal packages)
that does two things: proxies/serves a video stream to a browser, and forwards
remote-control commands to a physical TV setup. There are three source files
besides `main.go`:

- [config.go](config.go) — config struct definitions (viper/`mapstructure` tags).
- [api.go](api.go) — Harmony Hub REST client (`powerStatus`/`turnOn`/`turnOff`/`request`).
- [skyq.go](skyq.go) — raw TCP client that speaks Sky Q's binary remote protocol directly (ported from https://github.com/dalhundal/sky-remote).

### Config loading and layering

`loadCfg()` in [main.go](main.go) builds config via viper in layers, each one
merged on top of the last:

1. `config/config.yaml` (required — base config)
2. `config/config-dev.yaml`, merged in only if `dev.enabled: true` in the base config
3. the file at `$CONFIG_FILE`, if that env var is set

Any key can also be overridden by an environment variable of the same name
(viper `AutomaticEnv`), e.g. `SKY_Q.HOST`, `PORT`. `config/config-local.yaml`
is gitignored for untracked local overrides.

### Control backends

`cfg.Control` selects one of two mutually exclusive remote-control backends,
switched on in `apiCall()` in [main.go](main.go):

- `skyq` — talks directly to a Sky Q box over TCP ([skyq.go](skyq.go)), no external dependency.
- `harmony` — forwards commands as HTTP calls to a separately-run Harmony API instance ([api.go](api.go)), which in turn drives a Logitech Harmony Hub.

Button/command names (e.g. `channel-up`, `power`, digit keys) are the shared
vocabulary between the frontend and both backends — the SkyQ command map
lives in [skyq.go](skyq.go), Harmony forwards the name verbatim as a URL segment.

### Streaming

Video is not proxied through Go in production — quality-specific `.ts`/`.m3u8`
URLs from `stream.quality`/`stream.hls` config point directly at an external
streamer (e.g. reverse-proxied via nginx, see `samples/nginx/`). Go only
builds the master HLS playlist (`hlsPlaylist()` in [main.go](main.go), served
at `/playlist.m3u8`) from `stream.hls` config entries.

`dev.enabled: true` turns on a Go-side reverse proxy (`NewProxy`/
`ProxyRequestHandler` in [main.go](main.go)) that forwards `*.ts`/`*.m3u8`/`*.flv`
requests to `dev.stream`, working around browser CORS when developing locally
against a real streamer. This path must stay dev-only, not used in production.

### Frontend

Server-rendered via `html/template` (see `home`/`video`/`remote` handlers in
[main.go](main.go)), templates in [html/](html/) are composed by nesting
(`video.html` and `remote-home.html` both embed `remote.html`). No build step —
plain ES modules loaded via `<script type="module">`:

- [html/static/slinky.js](html/static/slinky.js) — entrypoint, wires up keyboard shortcuts, calls into the other two modules.
- [html/static/video.js](html/static/video.js) — player logic (hls.js for `/playlist.m3u8`, mpegts.js for direct `.ts` streams), fullscreen UI, casting.
- [html/static/remote.js](html/static/remote.js) — remote button handling, channel-number entry, mute/volume.
- [html/static/api.js](html/static/api.js) — thin fetch wrapper for `/api/v1/*`.

Bootstrap 5 and Font Awesome are loaded from CDN in the templates, not vendored.

## Notes

- [BACKLOG.md](BACKLOG.md) holds not-yet-planned future work (React rewrite, Sky Stream migration) — informational only, don't act on it unless asked.
- Sky Q protocol and Harmony forwarding are both fire-and-forget-ish command sends with no retry logic; keep new control-path code consistent with that (errors are surfaced as HTTP 500s, not retried).
