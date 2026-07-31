# Streamly (Redux)

A discord in-call Activity for watching Shows, Movies, and Live TV in a voice channel.

## Stack

- Backend: Go, Gin
- Frontend: React, TypeScript, Vite, Bun
- Discord Embedded App SDK

## Config

Copy `.env` and set:

```
DISCORD_CLIENT_ID=
DISCORD_CLIENT_SECRET=
LISTEN_ADDR=:8080
STATIC_DIR=web/dist
```

`ALLOW_ANY_ORIGIN=1` relaxes the proxy origin check for local testing.

## Run

Backend:

```
go run ./cmd/streamly
```

Frontend (dev):

```
cd web
bun install
bun run dev
```

Build SPA for production:

```
cd web
bun run build
```

The activity generally cannot use localhost. Discord loads it through a URL mapping to a public HTTPS host (tunnel the Vite port during development).

## Layout

```
cmd/streamly/     entrypoint
internal/         Go packages (auth, config, proxy, server, sources)
web/              React SPA
_docs/            design notes
```
