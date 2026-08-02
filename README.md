# Streamly (Redux)

A discord in-call Activity for watching Shows, Movies, and Live TV in a voice channel.

## Stack

- Backend: Go, Gin
- Frontend: React, TypeScript, Vite, Bun
- Discord Embedded App SDK

## Config

Copy `.env.example` to `.env` and fill it in. Only the Discord pair is required to boot; the
rest gates one feature each:

| | |
|---|---|
| `DISCORD_CLIENT_ID` / `DISCORD_CLIENT_SECRET` | Required — OAuth code exchange |
| `BOT_TOKEN` | Optional — bot stays online while the server runs; registers global `/launch` |
| `FEBBOX_UI_COOKIE` | VOD. Without it, movies and series report an authentication failure |
| `TMDB_API_KEY` | Home-page curation and high-quality poster/backdrop metadata |
| `SUBDL_API_KEY` | Subtitles. Without it, the subtitle menu stays empty |
| `INTRODB_TOKEN` | Skip intro. Optional; the anonymous tier works but is rate limited |
| `LISTEN_ADDR`, `STATIC_DIR` | Default `:8080`, `web/dist` |

`ALLOW_ANY_ORIGIN=1` relaxes the proxy origin check for local testing.

Upstream base URLs are deliberately not configuration — they live as constants beside the
parsers that depend on them.

## Channel catalog

There is nothing to maintain. iptv-org supplies channel identity, country, categories, and
logos; provider listings are matched onto it at startup and every six hours. A channel
appears only once at least one provider has been matched to it, and matching refuses
ambiguity — titles it cannot identify are quietly dropped rather than guessed at.

The proxy reports upstream failure to the room, which advances to the next source and
announces it. A channel gains a fallback whenever two provider entries match the same
canonical channel.

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

Tests:

```
go test ./...
cd web && bun test
```

The activity generally cannot use localhost. Discord loads it through a URL mapping to a public HTTPS host (tunnel the Vite port during development).

## Layout

```
cmd/streamly/     entrypoint
internal/
  auth/           OAuth exchange, token verification
  bot/            Discord gateway presence + global /launch
  catalog/        iptv-org reference, provider matching, refresh
  config/         .env loading
  proxy/          media relay, HLS rewriting, images, failure detection
  resolve/        room item to playable URL
  room/           hub, room state, sync protocol
  server/         routes, middleware, SPA fallback
  sources/        daddylive, ntv, showbox, febbox, subdl, introdb, tvmaze
web/              React SPA
_docs/            design notes
```
