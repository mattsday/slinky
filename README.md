# Slinky

I used to love my Slingbox back in the day. They killed it over and over (ads, web client, ...), and finally killed it
for good by shutting down the servers.

I've long wanted to build my own replacement and this is kind of step #1. It's very much a passion-project and something
I plan on working on over the coming years - exclusively for my own needs!

## Dependencies

1. [Logitch Harmony Hub](https://www.logitech.com/en-gb/products.html) - this controls the remote, activities etc
2. [Harmony API](https://github.com/maddox/harmony-api) - a REST API to control the Harmony hub
3. [URayCoder HD](https://www.amazon.co.uk/gp/product/B07D78L3SZ/ref=ppx_yo_dt_b_asin_title_o01_s00) or similar product
   that streams H.264/H.265 video
4. [Nginx](https://www.nginx.com/) or some kind of reverse proxy if you wish to watch video in your browser

## Setup

Assuming you have the dependencies working, you can deploy slinky using `samples/docker-compose.yaml`.

There are some configuration paramters you can change via `config/config.yaml` or environment variables (see below).

Once running you can go to `/video` for a live video view, or just the default page on `/` for a remote control.

### Harmony API

The Harmony API does most of Slinky's work - you can find an example docker-compose file
in `samples/harmony-api/docker-compose.yaml`

### Reverse Proxy

To make this work on the internet I strongly recommend using a reverse proxy and some kind of
SSO. `samples/nginx/slinky.example.conf` is a basic implementation. For my own personal use, I combine this
with [Nginx SSO by Luzifer](https://github.com/Luzifer/nginx-sso).

## Configuration

### Basic Config

| Config | Environment Variable | Description                        | Example     |
| ------ | -------------------- | ---------------------------------- | ----------- |
| `port` | `PORT`               | Port for this service to listen on | `PORT=8080` |

### Configure Harmony API

Configure the [Harmony API](https://github.com/maddox/harmony-api) and how it should connect.

| Config                         | Environment Variable           | Description                                                 | Example                                 |
| ------------------------------ | ------------------------------ | ----------------------------------------------------------- | --------------------------------------- |
| `harmony_api.url`              | `HARMONY_API.URL`              | Where the Harmony API is running                            | `HARMONY_API.URL=http://localhost:8282` |
| `harmony_api.default_hub`      | `HARMONY_API.DEFAULT_HUB`      | The hub to connect to (future versions may allow more flex) | `HARMONY_API.DEFAULT_HUB=living-room`   |
| `harmony_api.default_activity` | `HARMONY_API.DEFAULT_ACTIVITY` | The activity to use (future versions may allow more flex)   | `HARMONY_API.DEFAULT_ACTIVITY=watch-tv` |

### Streaming

Configure the built-in video stream

| Config        | Environment Variable | Description                                                                                       | Example             |
| ------------- | -------------------- | ------------------------------------------------------------------------------------------------- | ------------------- |
| `stream.hq`   | `STREAM.HQ`          | The default HQ stream URL to use                                                                  | `STREAM.HQ=/0.ts`   |
| `stream.fast` | `STREAM.FAST`        | Unused today, but future versions may switch to the fast stream when using the remote for example | `STREAM.FAST=/1.ts` |

### Dev

When developing locally CORS will stop you from directly streaming the video source into the browser. Go has some very
simple reverse proxy features to allow this, but we don't wanna run this in production - so useful for local IDE
development.

If you enable this, you should set `STREAM.HQ=/<x>.ts`.

| Config        | Environment Variable | Description                                       | Example                            |
| ------------- | -------------------- | ------------------------------------------------- | ---------------------------------- |
| `dev.enabled` | `DEV.ENABLED`        | Whether to enable Dev mode (reverse proxy) or not | `DEV.ENABLED=true`                 |
| `dev.stream`  | `DEV.STREAM`         | Your stream URL to proxy                          | `DEV.STREAM=http://192.168.1.168/` |

## Future Plans

When created I was a backend developer and had no clue about UI, UX, etc. If I were to do this again, I'd rewrite it in Next.js or possibly rewrite the frontend in Flutter so I could have it as a fat client in a browser. Oh well.
