# Streamly Redux — Design

Redesign of the Streamly bot around Discord's **In-Call Activities** (Embedded App SDK), replacing the previous architecture in which a worker user account joined a voice channel and streamed a downloaded HLS feed over UDP.

Status: slices 0 and 1 implemented (§11). Playback through the proxy is verified end to end; the sync service, catalog, and VOD path are not built yet.

---

## 1. Why this replaces the old design

The old bot transcoded media server-side and pushed it into a voice channel through a headless user account. That approach carried a permanently at-risk account, an ffmpeg process per concurrent session, a 720p ceiling from Go Live, no seeking, no subtitle tracks, and a dependency on undocumented voice-gateway behaviour.

Under Activities, each viewer's Discord client renders the video itself in a sandboxed iframe. Playback is native: full resolution, real seeking, real text tracks, per-user volume. Server CPU per viewer approaches zero.

What the redesign does **not** solve is upstream fragility. Scraping DaddyLive and NTV is exactly as brittle as it was before. That work carries over unchanged.

### What it costs

- **No passive viewing.** A Go Live stream is visible to anyone who clicks it. An Activity must be launched and joined. Participation is opt-in per user.
- **No fan-out.** Every viewer pulls the full stream independently. Ten viewers is ten times the bytes.
- **No chat presence.** With no bot (§2.5) there are no "now playing" messages, no announcements, and no way to queue anything without opening the activity.

---

## 2. Architecture

One Go binary and one React SPA, on one host, behind one Discord URL mapping.

```
Discord client (iframe)
  │
  │  all traffic transits <app_id>.discordsays.com
  ▼
VPS ── Go binary
        ├── /              static SPA
        ├── /api/*         metadata, OAuth token exchange
        ├── /ws            sync WebSocket
        └── /proxy/*       media proxy
```

### 2.1 URL mapping

A single root mapping (`/` → the VPS host) covers every path. Because the activity is served *from* that mapping, all relative requests — `/api/...`, `/ws`, `/proxy/...` — resolve through Discord's proxy without any call to `patchUrlMappings()`. No per-source mappings are needed, because no client ever contacts an upstream source directly.

**This holds only because images route through the proxy too.** The activity CSP blocks third-party image hosts exactly as it blocks any other external request, so Showbox posters, iptv-org channel logos, and Discord avatars would all fail against a naive `<img src>` pointing at their origin. Sending them through `/proxy` keeps the mapping count at one and means a source changing CDN never requires a dev-portal edit.

**Every byte of video transits Discord's proxy.** This is unavoidable in the activity sandbox and is worth stating plainly: it is the operational risk that replaces the old design's account-ban risk. Sustained multi-gigabyte relay through `discordsays.com` is visible to Discord in a way the old architecture was not.

### 2.2 Media proxy

Deliberately thin. It exists because the browser cannot do three things:

1. **Set forbidden headers.** iptv-org-style streams and DaddyLive both gate on `Referer` and `User-Agent`, which `fetch` refuses to set. The proxy sets them per source.
2. **Escape CORS.** Upstream hosts send no permissive CORS headers. The proxy does.
3. **Rewrite HLS manifests.** Segment URLs inside `.m3u8` responses are absolute and must be rewritten to point back through `/proxy/`.
4. **Load third-party images.** Posters, channel logos, and avatars are blocked by CSP at their origin (§2.1), so they are fetched server-side and re-served.

Beyond that it forwards bytes and honours `Range`. There is **no ffmpeg and no transcoding**: Febbox's `video_quality_list` returns Febbox's own transcoded renditions (H.264/AAC MP4), not source files, so the browser can play them directly. Live sources are already browser-compatible HLS.

**Access control is an origin check only.** The proxy rejects requests that don't arrive from the `discordsays.com` origin. This stops casual abuse and nothing else — an origin header is trivially forged, so the proxy should be understood as an open relay with a speed bump, not a protected resource. Recorded as an accepted risk in §12.

A short-TTL in-memory segment cache for live is optional and worth adding only if several viewers commonly share a channel — it collapses N upstream pulls into one. VOD needs no cache; range requests against the origin are fine.

### 2.3 Sync service

In-memory only. A room is keyed by the Discord **activity instance ID** and holds:

```
item        what is playing (VOD ref or channel ref)
playing     bool
anchorMs    position at anchorAt
anchorAt    server unix ms
subtitle    selected track, or none
queue       ordered list — VOD only, see §4.6
lastActor   who last acted, and what they did
```

Quality is deliberately absent: it is a per-client concern (§4.7) and never enters room state.

Position is never streamed. Clients derive it:

```
expected = playing ? anchorMs + (serverNow - anchorAt) : anchorMs
```

`serverNow` comes from the client's own clock plus a measured offset (§4.3).

### 2.4 No database

Nothing survives a restart, and nothing needs to. Rooms are ephemeral, resume was cut from v1, and all content metadata is fetched on demand from upstream. A deploy is a binary restart that costs any live room its session — acceptable, since a room already dies when its last participant leaves.

The only persistent file in the system is the hand-maintained channel map (§5.2), which is configuration, not state.

### 2.5 No bot

No gateway connection, no command handlers, no shard management. The application still needs a bot *user* attached for guild installation, and the client secret is still required server-side for the OAuth code exchange — but nothing ever connects to the gateway.

---

## 3. Authentication

Scope required: `identify`. Nothing else.

```
sdk.ready()
  → sdk.commands.authorize({ scope: ["identify"] })   returns code
  → POST /api/token { code }                          server exchanges w/ client secret
  → sdk.commands.authenticate({ access_token })
  → WebSocket connect, first frame carries access_token
```

The sync service verifies the token against `/users/@me` once at connect, caches the resulting user ID and display name for the connection's lifetime, and never stores it beyond that.

**Participant presence comes from live WebSocket connections, not from the Discord SDK's participant list.** The two can disagree — someone can be in the voice channel without having opened the activity — and the connection list is the one that reflects who is actually watching.

---

## 4. Playback synchronisation

### 4.1 The governing rule

> **Deliberate actions propagate. Involuntary ones do not.**

A pause, play, seek, or track change is intent: it rewrites the anchor and broadcasts to the room. Buffering is not intent: it stays entirely local. The affected client shows its own spinner, and on recovery recomputes `expected` and seeks there silently.

This single rule replaces the entire apparatus that a watch-party system normally needs — no participant health tracking, no slowest-viewer computation, no drift thresholds negotiated across clients, no catch-up handshake.

### 4.2 Correction

A client compares actual playback position against `expected` on a timer, on tab focus, and after any stall. If they differ by more than **2 seconds**, it hard-seeks. Below that it does nothing.

No playback-rate nudging. Rate manipulation buys imperceptible correction at the cost of real complexity, and a hard seek every few minutes at most is not worth that trade here.

### 4.3 Clock offset

All position maths is wall-clock based, so a client with a skewed clock will sit confidently in the wrong place. At connect the client sends several `ping` frames and takes the median offset:

```
offset = t_server - (t_sent + rtt / 2)
```

Refreshed periodically, and after any reconnect. This is a genuine correctness requirement, not a refinement — it is the difference between the room being in sync and only believing it is.

### 4.4 Live streams

A live channel has no shared timeline, so `anchorMs` is meaningless. `expected` is simply *the live edge*. Recovery after a stall means jumping to the edge rather than to a computed position — same rule, no special-casing in the protocol.

Viewers will still sit a few seconds apart, set by their own buffer depth. That is accepted. Holding the room to its slowest participant was considered and rejected as not worth the latency and coupling.

Pause still works and still propagates — the governing rule holds. **Resuming returns the room to the live edge, not to the paused position.** Attempting to resume where the pause happened would depend on whether the interval is still inside the HLS window, which makes the behaviour a function of how long someone stepped away. Jumping to the edge always works, keeps the room together, and loses the interval honestly rather than unpredictably.

### 4.5 Control

Anyone in the room can pause, seek, skip, queue, and reorder. There are no roles, no host, and no handoff logic. Every action is attributed in the UI (§6.3), which is sufficient deterrent among a small trusted group and costs nothing to build.

### 4.6 Queue

**The queue is VOD-only. Live channels bypass it entirely.**

Tuning to a channel replaces whatever is playing and leaves the queue untouched; stopping the channel returns the room to the queue where it left off. `next` and autoplay have no meaning while a channel is playing and are inert in that state.

This keeps two genuinely different ideas apart. Tuning in is not queueing, and forcing channels into the queue would either stall it on an entry that never advances or require inventing an arbitrary duration for a live broadcast — which would cut off the end of a match, precisely the wrong failure for the sports use case this is being built for.

Within VOD, finishing an item advances automatically to the next. **The queue is pure "up next"** — starting a title (play-from-queue, autoplay, or skip) removes it from the list so the room is not left with a growing playlist of things already watched. Rewatch is re-queue or play again from browse. **Series roll across season boundaries** when those episodes are queued — the last episode of a season continues into the first of the next, since stopping there would interrupt exactly the binge the queue exists to support.

### 4.7 Personal versus shared settings

Both subtitles and quality render locally, so neither is forced to be shared. They are split by what each one actually is:

- **Subtitles are shared.** A subtitle choice is a decision about the content, and a room watching a foreign-language film should not each have to turn them on. Selection propagates like any other deliberate action.
- **Quality is personal.** A bitrate choice is a decision about *your* connection. One viewer on poor bandwidth should be able to drop to 480p without imposing that on everyone, and every client is pulling its own copy anyway, so nothing about the shared session depends on them matching.

Febbox returns discrete renditions rather than an adaptive ladder, so there is no ABR and a default must be chosen outright: **the highest rendition at or below 1080p**. 4K is excluded deliberately — the activity pane is far too small for the difference to be visible, and every byte relays through Discord's proxy (§2.1), so the cost is real and the benefit is not.

---

## 5. Content sources

### 5.1 Priority

1. **DaddyLive** — channel-based live TV including sports. First to build.
2. **NTV** — direct-manifest channels only, as failover for DaddyLive.
3. **Showbox / Febbox** — VOD, movies and series, with queue and episode autoplay.

Full sports *listings* (NTV `get-matches`) are explicitly deferred past v1. Pluto and iptv-org streams are out of scope — though iptv-org's `logos.json` is retained purely as a logo lookup (§5.3).

**On NTV scope.** Some NTV channels expose a manifest directly; others hide it behind an obfuscated JS iframe embed that would have to be deobfuscated to reach the stream at all. **Only the first kind is in scope.** Deobfuscation is explicitly not being done — it is the most fragile possible dependency, it breaks on every upstream cosmetic change, and the channels behind it are not worth that maintenance. Channels of the second kind are simply absent from `channels.yaml`.

This means classification is a curation decision made once per channel when it is added to the map, not a runtime probe. The direct-manifest channels work with the same header treatment every other source needs (§2.2).

### 5.2 Failover

DaddyLive exposes `data-title`; NTV exposes `channel_name` and `channel_code`. **Nothing links them.** Fuzzy matching across sports channel naming will produce wrong matches, and an announced failover that silently shows a different channel is worse than an error.

Therefore: a hand-maintained `channels.yaml` mapping DaddyLive channels to their NTV equivalents, covering only channels actually watched. Unmapped channels simply have no fallback and fail honestly.

```yaml
- id: espn-us
  name: ESPN
  sources:
    - provider: daddylive
      ref: "123"
    - provider: ntv
      ref: "espn"
```

**Failure detection lives in the proxy, not the client.** The proxy sees upstream 4xx/5xx and dead segments directly; that observation is authoritative and room-wide. Client-side detection would require deciding whether one person's failure means everyone should switch — a vote, effectively. The proxy makes that question disappear: it notifies the room hub, the hub advances to the next source, and every client receives a `notice` frame rendered as a brief *"switched to backup source"* toast.

Announcing rather than hiding the switch also makes upstream rot visible in logs, which is how you find out a source has died before users report it.

### 5.3 Channel artwork

DaddyLive and NTV supply no logos, and a channel grid without them is a wall of text. iptv-org's `logos.json` is fetched once at startup and matched by channel name into the same `channels.yaml` entries.

### 5.4 Showbox / Febbox notes

- The 3DES envelope, key, and IV are documented in [APIs.md](APIs.md) and carry over unchanged. These have historically been stable — Showbox is the least troublesome source in the system.
- **The Febbox `ui` cookie remains a single point of failure, but a low-churn one.** It is a per-account token with roughly a year's lifetime, supplied manually. `video_quality_list` does not work without it, so its eventual expiry stops all VOD at once — which makes the error state the thing that matters: an empty quality list rendered as "no qualities available" will send you debugging the wrong layer entirely. Authentication failure must surface as authentication failure.
- Episode listings come from `TV_detail_v2`, with TVMaze as a secondary source when the Showbox episode list is incomplete.

**VOD has no cross-source failover** — there is no second provider for a given title, so §5.2 does not apply. It does, however, have alternatives *within* a title: a mid-playback failure is usually a dead rendition URL rather than a dead title. On failure the client retries a few times (many are transient), then re-resolves the quality list and presents the remaining renditions as alternatives at the current position. That recovers the common case without abandoning what the room was watching.

### 5.5 Subtitles

SubDL returns SRT and ZIP archives, neither of which a browser can use. The proxy unpacks and converts to WebVTT, then serves it as a real `<track>`. This is the only content transform anywhere in the system.

SubDL usually returns many releases per title. Candidates are scored against the release name and the best match is loaded automatically, but **subtitles default to off** — they are a shared setting (§4.7), so enabling them by default would put text on everyone's screen because one release happened to match well.

The realistic failure is a wrong-release match producing subtitles that drift out of time. The full release list therefore stays available behind the subtitle menu, so someone can switch without leaving playback.

**Two calls, two very different budgets.** `GET /subtitles?api_key=…` takes an IMDB or TMDB id plus optional season and episode; downloads then come from `dl.subdl.com` and are unauthenticated on the free tier. The free allowance is roughly 2,000 searches/day but only **50–300 downloads/day** — searching is effectively unlimited at this scale, downloading is not.

Converted WebVTT is therefore cached in memory, keyed by subtitle path, so switching tracks or rewatching within a session costs nothing. Cache is lost on restart, which at a handful of viewers stays comfortably inside the daily allowance; an on-disk cache is the upgrade path if the download budget ever actually binds, and would be the only persistent state in the system (§2.4).

### 5.6 Skip intro

IntroDB is queried by `tmdb_id`/`imdb_id`, both available from `Movie_detail` and `TV_detail_v2`. Returned ranges drive a skip button. Because skipping is a deliberate seek, it propagates — one person skips, the room skips.

The bearer token is used rather than the anonymous tier. Autoplaying through a season means one lookup per episode, which is a meaningfully higher request rate than the old design ever produced, and authenticating is free insurance against hitting an anonymous limit mid-binge.

---

## 6. Interface

React + TypeScript + Tailwind + shadcn/ui, built with Bun and Vite. Function components throughout; see [CLAUDE.md](../CLAUDE.md), updated for this decision.

Desktop-first. Layouts stay responsive and mobile is expected to work where the webview cooperates, but it is not tested or guaranteed — iOS webview media playback is the least predictable surface in the whole system and is not worth blocking v1 on.

### 6.1 Modes

Two distinct full-frame modes, switched explicitly:

- **Browse** — opens on a unified home: live channels, trending from `Search_hot`, and the current queue on one screen. One search field spans channels and VOD together, results grouped by kind — someone who knows what they want rarely cares which source it lives on.
- **Watch** — the player, full frame.

**Opening the activity while the room is already watching lands directly in the player, already synced,** with a dismissible card naming the title and who else is here. Someone arriving mid-session is walking into a room where the TV is on; making them click past a lobby every time is friction for no benefit. The card exists only so they know what they've walked into.

Audio starts muted until the new arrival interacts. Browsers generally require a gesture before playing audio anyway, and unmuting into an active voice call unannounced is the wrong first impression.

The activity pane is small. Distinct modes were chosen over a persistent sidebar precisely so that neither screen has to fight for space.

### 6.2 Player chrome

Standard auto-hiding controls: play/pause, scrubber, volume, subtitles, quality, fullscreen.

Persistently visible alongside them: **presence** — who is in the room, and who last acted. This is the one place the UI departs from a normal solo player, and it is what makes an unexpected pause legible rather than mysterious.

The subtitle and quality controls sit side by side but do different things (§4.7), and the UI has to say so — changing subtitles affects everyone, changing quality affects only you. Without that cue the two adjacent menus look identical and one of them will surprise someone.

### 6.3 Notices

One lightweight toast channel carries all room-level events: source failover, someone's action attribution, a viewer joining or leaving. Deliberately minimal — the room's voice chat is the real communication layer.

---

## 7. Module layout

Repository directory `streamly-redux`; Go module **`streamly`**. The shorter module name keeps import paths readable (`streamly/internal/room`) and matches the internal-package prefix already documented in [CLAUDE.md](../CLAUDE.md).

### 7.1 Backend — Go, Gin

```
go.mod                            module streamly
cmd/streamly/main.go              wiring only — config, deps, serve

internal/config/                  .env loading, typed Config
internal/server/                  gin engine, routes, middleware, SPA fallback
internal/auth/                    OAuth code exchange, token verification, user lookup

internal/room/
  hub.go                          instance registry, lifecycle
  room.go                         one room's state and transitions
  protocol.go                     wire types (§8)
  conn.go                         per-connection read/write pumps

internal/proxy/
  handler.go                      routing, range passthrough, origin check
  headers.go                      per-source header injection
  hls.go                          manifest rewriting
  image.go                        third-party image fetch (§2.1)
  detect.go                       upstream failure detection (§5.2)

internal/catalog/                 channel model, channels.yaml, logo matching
internal/resolve/                 room item → playable URL, across providers

internal/sources/showbox/
  envelope.go                     3DES request envelope
  client.go
  search.go
  detail.go
internal/sources/febbox/          share key, file list, quality list
internal/sources/daddylive/
internal/sources/ntv/
internal/sources/subdl/
  client.go
  vtt.go                          SRT/ZIP → WebVTT
  cache.go                        in-memory, download-budget aware (§5.5)
internal/sources/introdb/
```

`catalog` and `resolve` are deliberately separate: one answers *what can be watched*, the other *how to play this specific thing right now*. Merging them would put YAML parsing and HTTP resolution in the same package for no gain.

Gin handles routing, the static SPA directory, and middleware. It is not used for anything below the HTTP layer — `room`, `proxy`, and every source package stay free of framework types so they remain testable without a request.

### 7.2 Frontend — React, Bun, Vite

Bun as both package manager and runtime for tooling. Not Node, not Deno.

```
web/
  package.json
  vite.config.ts
  src/
    main.tsx
    App.tsx                       mode switching, top-level providers

    lib/sdk.ts                    Embedded App SDK init and auth handshake
    lib/sync.ts                   WebSocket client, reconnect
    lib/clock.ts                  offset estimation, expected-position maths (§4.3)
    lib/api.ts
    lib/cn.ts

    hooks/useRoom.ts              room state subscription
    hooks/usePlayer.ts            element binding, drift correction (§4.2)
    hooks/useParticipants.ts

    components/ui/                vendored shadcn — exempt from house style
    components/player/            Player, Controls, Presence, SubtitleMenu, QualityMenu
    components/browse/            SearchBar, ChannelGrid, TitleGrid, Queue

    screens/Browse.tsx
    screens/Watch.tsx
```

Tailwind for everything beyond what shadcn primitives already provide. `lib/clock.ts` is split out from `lib/sync.ts` on purpose — the position arithmetic is the one piece of frontend logic worth unit-testing in isolation, and it should not require a WebSocket to exercise.

### 7.3 Sizing

A package should be describable in a noun phrase without "and". When a file needs two clauses to explain, it wants splitting; roughly 200–300 lines is the point where that usually becomes true. Source packages each expose a small typed surface and keep their scraping and parsing private — nothing outside `internal/sources/daddylive` should know that it parses HTML.

---

## 8. Sync protocol

JSON frames over one WebSocket.

**Client → server**

| Type | Payload |
|---|---|
| `hello` | `instanceId`, `accessToken` |
| `ping` | `t0` |
| `control` | `action`: `play` \| `pause` \| `seek` \| `next` \| `prev` \| `setItem` \| `setSubtitle`, optional `positionMs`, optional `item`, optional `track` |
| `queue` | `op`: `add` \| `remove` \| `move`, plus operands |

Quality selection has no frame. It never leaves the client.

**Server → client**

| Type | Payload |
|---|---|
| `welcome` | full room state, participants, `serverTime` |
| `pong` | `t0`, `t1` |
| `room` | full room state — broadcast on any change |
| `participants` | connected users |
| `notice` | `kind`: `failover` \| `error` \| `action`, plus display text |

Room state is always sent whole. It is a few hundred bytes and changes rarely; diffing it would add a class of bug for no measurable gain.

---

## 9. Local development

**An Activity cannot run against `localhost`.** Discord serves the app from `<app_id>.discordsays.com` and fetches it through the URL mapping, so the mapping target must be a publicly reachable HTTPS host. There is no loopback exemption and no offline mode. This is the first thing that will bite during slice 0 and it is worth knowing before rather than after.

The working setup:

| | |
|---|---|
| Public origin | `https://streamly-redux.sprout.software` |
| Tunnel target | `localhost:9090` |
| Vite dev server | `:9090` — serves the SPA, proxies `/api`, `/ws`, `/proxy` to Go |
| Go binary | `:8080` — `LISTEN_ADDR` in `.env` |

`https://streamly-redux.sprout.software` is the single root URL mapping in the dev portal, and the tunnel terminates at Vite on **9090**. HMR and WebSockets both survive the tunnel, so the normal edit-reload loop is intact.

**Because the hostname is stable and will also serve production, one Discord application is enough.** Deployment repoints the hostname from the tunnel to the VPS; the mapping, client ID, and secret never change. That removes the two-applications-and-two-credential-pairs arrangement this section previously called for — worth noting only because it is a real simplification, not an oversight.

The consequence is that local and production cannot be live at the same time on this hostname. If that becomes annoying, a second subdomain and a second application is the fix, and nothing in the design resists it.

The VPS is not needed until deployment; everything above runs on the development machine.

---

## 10. Operations

**Serving.** Gin serves the built SPA from a directory on disk (`bun run build` output) and handles `/api`, `/ws`, and `/proxy` itself. No nginx, no Caddy, no embedded assets — the frontend can be redeployed without rebuilding Go. The cost is two artifacts that must stay in step, and a static path in config. TLS is terminated wherever the domain is fronted; a certificate is mandatory regardless, since the URL mapping requires HTTPS.

**Config.** A gitignored `.env` loaded at startup, holding **credentials and environment only**:

```
DISCORD_CLIENT_ID
DISCORD_CLIENT_SECRET
FEBBOX_UI_COOKIE
SUBDL_API_KEY
INTRODB_TOKEN
LISTEN_ADDR
STATIC_DIR
```

**Upstream base URLs are not configuration.** DaddyLive rotates domains and cannot be predicted, so its base URL — and every other source's — lives as a constant in its own source package, updated in a commit when it breaks. Putting them in `.env` would imply an operator can fix a rotation by editing config, when in reality a rotation usually changes the page structure too and needs a code change regardless. Keeping them in code puts the URL next to the parser that depends on it.

The channel map (§5.2) stays a separate YAML file: curated data rather than configuration, and worth having diffable.

**Logging.** `log/slog` to stdout, structured. Upstream failures and failovers log loudly and distinctly — per §5.2 that is the single most valuable thing to see, because it is how you learn a scraper has rotted before anyone reports it. Everything else stays quiet.

---

## 11. Build order

Each slice is independently demonstrable. The ordering front-loads risk deliberately — slices 0 and 1 are where this design either works or doesn't, and both are small.

| # | Slice | Proves |
|---|---|---|
| 0 | Activity boots, OAuth completes, renders the user's name | Dev portal config, URL mapping, handshake — the whole platform assumption |
| 1 | One hardcoded DaddyLive channel plays through the proxy | The riskiest thing in the project: CSP, header injection, HLS rewriting, and codec support all at once |
| 2 | Two clients, play/pause/seek propagates, clock offset works | The sync model |
| 3 | Channel catalog, logos, browse mode | Catalog plumbing |
| 4 | Showbox/Febbox VOD, queue, episode autoplay | The VOD path end to end |
| 5 | Subtitles, skip intro | The only content transforms |
| 6 | Failover, notices, presence, polish | Resilience and the shared-session feel |

**Do not build past slice 1 until slice 1 works.** Everything downstream assumes the proxy can deliver playable video into the sandbox, and that assumption is cheap to test and expensive to be wrong about.

---

## 12. Open risks

| Risk | Assessment |
|---|---|
| All video relays through Discord's proxy | Unavoidable. Replaces account-ban risk with app-termination risk. |
| DaddyLive/NTV have no join key | Mitigated by hand-maintained mapping; does not scale past curated channels. |
| Febbox `ui` cookie is a shared SPOF | Needs a refresh path and a legible failure state. |
| NTV encrypted-stream filtering unverified | Needs a probe step to classify streams before offering them. Unknown how cleanly they separate. |
| Proxy is effectively an open relay | Accepted. The origin check (§2.2) is forgeable; anyone who obtains a URL can use your bandwidth. Signed short-lived URLs are the upgrade path if it ever matters. |
| Upstream scraping rots | Inherited from the old design, not introduced by it. Announced failover surfaces it early. |
| iOS webview playback | Accepted; mobile is best-effort. |
| Restart drops live rooms | Accepted; rooms are ephemeral by design. |

### Unresolved inputs

Everything the design was waiting on is now answered and folded into the sections above — NTV scope (§5.1), the Febbox cookie (§5.4), DaddyLive domain handling (§10), Showbox key stability (§5.4), SubDL limits (§5.5), the IntroDB token (§5.6), and naming (§7).

One item remains, and it is credentials rather than design: the `.env` values in §10 must be populated before the slices that use them. Only `DISCORD_CLIENT_ID` and `DISCORD_CLIENT_SECRET` are needed for slice 0.
