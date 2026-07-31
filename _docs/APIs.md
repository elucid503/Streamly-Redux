# Content APIs

## Showbox

**Base:** `https://mbpapi.shegu.net/api/api_client/index/`  
**Method:** `POST` `application/x-www-form-urlencoded`  
**Body:** `appid`, `platform`, `version`, `medium`, `data` (base64 envelope)

### Envelope (before base64)

```json
{
  "app_key": "<md5(app_key_string)>",
  "verify": "<md5(md5(app_key_string) + 3des_key + encrypt_data)>",
  "encrypt_data": "<3DES-CBC base64 ciphertext of request JSON>"
}
```

### Request JSON (decrypted)

```json
{
  "module": "<name>",
  "childmode": "0",
  "APP_VERSION": "11.5",
  "LANG": "en",
  "PLATFORM": "android",
  "CHANNEL": "Website",
  "APPID": "27",
  "VERSION": "129",
  "MEDIUM": "Website",
  "expired_date": 0
}
```

Plus module-specific fields. Crypto: 3DES-CBC key `123d6cedf626dy54233aa1w6`, IV `wEiphTn!`, PKCS7. `app_key` string is `moviebox`.

### Response wrapper

```json
{
  "data": {}
}
```

`data` is the module payload below.

---

### `Search5`

```json
{ "keyword": "string", "type": "all|movie|tv", "page": 1, "pagelimit": 20 }
```

```json
[
  {
    "id": 0,
    "box_type": 1,
    "title": "string",
    "year": 0,
    "poster": "string",
    "description": "string",
    "imdb_rating": "string"
  }
]
```

`box_type`: `1` movie, `2` series.

---

### `Search_hot`

```json
{ "type": "movie|tv", "pagelimit": 25 }
```

```json
["keyword", "..."]
```

---

### `Top_list`

```json
{ "box_type": 1 }
```

```json
[{ "id": "string", "display_name": "string" }]
```

---

### `Top_list_movie` / `Top_list_tv`

```json
{ "id": "list_id", "page": 1, "pagelimit": 20 }
```

Same item schema as `Search5`.

---

### `Movie_detail`

```json
{ "mid": 0 }
```

```json
{
  "title": "string",
  "year": "string",
  "poster": "string",
  "poster_org": "string",
  "poster_min": "string",
  "description": "string",
  "imdb_rating": "string",
  "tmdb_id": 0,
  "imdb_id": "string"
}
```

(Additional fields may be present.)

---

### `TV_detail_v2`

```json
{ "tid": 0 }
```

```json
{ "tid": 0, "season": 1 }
```

```json
{
  "title": "string",
  "year": "string",
  "poster": "string",
  "poster_org": "string",
  "poster_min": "string",
  "description": "string",
  "imdb_rating": "string",
  "tmdb_id": 0,
  "imdb_id": "string",
  "episode": [
    {
      "season": 1,
      "episode": 1,
      "title": "string"
    }
  ]
}
```

(Additional fields may be present.)

---

## Showbox media (share link)

**Base:** `https://www.showbox.media`

### `GET /index/share_link?id={id}&type={1|2}`

```json
{
  "data": {
    "link": "https://…/share/{share_key}"
  }
}
```

`type`: `1` movie, `2` series. Share key = last path segment of `link`.

---

## Febbox

**Base:** `https://www.febbox.com`  
**Auth:** cookie `ui=<token>` (required for quality list)

### `GET /file/file_share_list?share_key={key}&pwd=&parent_id={id}&is_html=0`

```json
{
  "data": {
    "file_list": [
      {
        "fid": 0,
        "file_name": "string",
        "is_dir": 0
      }
    ]
  }
}
```

`is_dir`: `0` file, `1` directory. Root: `parent_id=0`.

---

### `GET /console/video_quality_list?fid={fid}`

```json
{
  "html": "<markup with file_quality elements>"
}
```

Parsed from HTML attributes/text (not pure JSON fields):

| Field | Source |
|---|---|
| `url` | `data-url` |
| `quality` | `data-quality` |
| `name` | `.name` text |
| `size` | `.size` text |
| `speed` | `.speed span` text |

Logical item:

```json
{
  "url": "https://…",
  "quality": "string",
  "name": "string",
  "size": "string",
  "speed": "string"
}
```

---

## TVMaze

**Base:** `https://api.tvmaze.com`

### `GET /lookup/shows?imdb={imdb_id}`

```json
{
  "id": 0
}
```

(Full show object; only `id` required.)

---

### `GET /shows/{id}/episodes`

```json
[
  {
    "season": 1,
    "number": 1,
    "name": "string"
  }
]
```

---

## iptv-org (channel catalog)

### `GET https://iptv-org.github.io/api/channels.json`

```json
[
  {
    "id": "ESPN.us",
    "name": "string",
    "alt_names": ["string"],
    "network": "string|null",
    "owners": ["string"],
    "country": "US",
    "categories": ["string"],
    "is_nsfw": false,
    "launched": "string|null",
    "closed": "string|null",
    "replaced_by": "string|null",
    "website": "string|null"
  }
]
```

---

### `GET https://iptv-org.github.io/api/logos.json`

```json
[
  {
    "channel": "ESPN.us",
    "in_use": true,
    "width": 0,
    "height": 0,
    "format": "string",
    "url": "https://…"
  }
]
```

---

### `GET https://iptv-org.github.io/api/streams.json`

```json
[
  {
    "channel": "ESPN.us",
    "feed": "string",
    "title": "string",
    "url": "https://….m3u8",
    "quality": "string",
    "user_agent": "string",
    "referrer": "string"
  }
]
```

---

## NTV

**Base:** `https://ntv.cx`

### `GET /api/get-channels`

```json
{
  "success": true,
  "channels": [
    {
      "channel_id": "string",
      "channel_name": "string",
      "channel_code": "string",
      "channel_url": "https://…",
      "server": "cdnlive|…"
    }
  ]
}
```

`channel_url` is a player HTML page. Stream URL is extracted from page JS (not a fixed JSON field).

---

### `GET /api/get-matches?server=kobra&type=both`

```json
{
  "success": true,
  "live": [/* match */],
  "nonLive": [/* match */],
  "all": [/* match */]
}
```

Match:

```json
{
  "id": "string",
  "title": "string",
  "category": "string",
  "date": 0,
  "live": false,
  "teams": {
    "home": { "name": "string" },
    "away": { "name": "string" }
  }
}
```

`date` = Unix ms.

---

### `GET /watch/kobra/{match_id}`

HTML. Broadcaster labels from `#sourceSelect` `<option>` text (not JSON).

---

## DaddyLive

**Base:** `https://dlhd.st`

| URL | Response |
|---|---|
| `GET /24-7-channels.php` | HTML; cards: `/watch.php?id={n}`, `data-title` |
| `GET /watch.php?id={id}` | HTML; stream embed iframe |
| `GET /stream/stream-{id}.php` | HTML; premiumtv iframe |
| `GET {premiumtv player URL}` | HTML; `source: window.atob('…')` → HLS URL |

---

## Pluto TV

### `GET https://boot.pluto.tv/v4/start`

Query: `appName=web`, `appVersion`, `deviceVersion`, `deviceModel`, `deviceMake`, `deviceType`, `clientID`, `clientModelNumber`, `serverSideAds`, `constraints`.

```json
{
  "sessionToken": "string",
  "stitcherParams": "string",
  "servers": {
    "stitcher": "https://…"
  }
}
```

---

### `GET https://service-channels.clusters.pluto.tv/v2/guide/channels`

Query: `channelIds=`, `offset`, `limit`, `sort=number%3Aasc`  
Header: `Authorization: Bearer {sessionToken}`

```json
{
  "data": [
    {
      "id": "string",
      "name": "string",
      "slug": "string",
      "stitched": {
        "path": "/stitch/hls/channel/{id}/master.m3u8"
      }
    }
  ]
}
```

HLS: `{stitcher}{path}?{stitcherParams}`  
Default path if missing: `/stitch/hls/channel/{id}/master.m3u8`

---

## SubDL

**API base:** `https://api.subdl.com/api/v1`  
**Download base:** `https://dl.subdl.com`

### `GET /subtitles`

| Query | |
|---|---|
| `api_key` | required |
| `languages` | `EN` |
| `unpack` | `1` |
| `type` | `movie` or `tv` |
| `imdb_id` or `tmdb_id` | one required |
| `season_number` | tv |
| `episode_number` | tv |
| `file_name` | optional |

```json
{
  "status": true,
  "error": "string",
  "subtitles": [
    {
      "release_name": "string",
      "name": "string",
      "url": "/path/…",
      "season": 0,
      "episode": 0,
      "hi": false,
      "unpack_files": [
        {
          "url": "/path/…",
          "name": "string",
          "release_name": "string",
          "season": 0,
          "episode": 0,
          "format": "string",
          "language": "string",
          "hi": false
        }
      ]
    }
  ]
}
```

### `GET https://dl.subdl.com{url}`

Binary subtitle file or archive. Path from `url` / `unpack_files[].url`.

---

## IntroDB

**Base:** `https://api.theintrodb.org/v3`  
**Auth:** optional `Authorization: Bearer {token}`

### `GET /media`

| Query | |
|---|---|
| `tmdb_id` | and/or |
| `imdb_id` | |
| `season` | optional |
| `episode` | optional |
| `duration_ms` | optional |

```json
{
  "tmdb_id": 0,
  "type": "string",
  "intro": [
    {
      "start_ms": 0,
      "end_ms": 0
    }
  ]
}
```
