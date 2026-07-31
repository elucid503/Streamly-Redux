package daddylive

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Rotates without warning; a rotation usually changes page structure too, so this lives in code (see _docs/DESIGN.md §10).
const baseURL = "https://dlhd.st"

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

const maxBodyBytes = 8 << 20

var (

	ErrStreamNotFound = errors.New("daddylive: no stream URL in embed chain")
	ErrChannelOffline = errors.New("daddylive: channel resolved but is not broadcasting")

)

// Ad banners inject their own iframes, so each hop matches its specific target rather than the first iframe on the page.
var (

	embedPattern = regexp.MustCompile(`(?i)src=["'](https?://[^"']*stream/stream-\d+\.php)["']`)
	playerPattern = regexp.MustCompile(`(?i)<iframe[^>]+src=["'](https?://[^"']*premiumtv[^"']+)["']`)
	sourcePattern = regexp.MustCompile(`(?i)source\s*:\s*window\.atob\(\s*['"]([A-Za-z0-9+/=]+)['"]\s*\)`)
	atobPattern = regexp.MustCompile(`(?i)atob\(\s*['"]([A-Za-z0-9+/=]+)['"]\s*\)`)

)

type Stream struct {

	URL string
	Referer string

}

type Client struct {

	http *http.Client

}

func New() *Client {

	httpClient := &http.Client{

		Timeout: 25 * time.Second,

	}

	return &Client{

		http: httpClient,

	}

}

func (c *Client) Resolve(ctx context.Context, channelID string) (*Stream, error) {

	watchURL := fmt.Sprintf("%s/watch.php?id=%s", baseURL, url.QueryEscape(channelID))

	watchBody, err := c.fetch(ctx, watchURL, baseURL+"/")

	if err != nil {

		return nil, err

	}

	embedURL := fmt.Sprintf("%s/stream/stream-%s.php", baseURL, channelID)

	if match := embedPattern.FindStringSubmatch(watchBody); match != nil {

		embedURL = match[1]

	}

	embedBody, err := c.fetch(ctx, embedURL, watchURL)

	if err != nil {

		return nil, err

	}

	playerURL := fmt.Sprintf("%s/premiumtv/daddy3.php?id=%s", baseURL, url.QueryEscape(channelID))

	if match := playerPattern.FindStringSubmatch(embedBody); match != nil {

		playerURL = match[1]

	}

	playerBody, err := c.fetch(ctx, playerURL, embedURL)

	if err != nil {

		return nil, err

	}

	encoded := findSource(playerBody)

	if encoded == "" {

		slog.Error("daddylive player had no source", "channel", channelID, "player", playerURL)

		return nil, ErrStreamNotFound

	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)

	if err != nil {

		return nil, fmt.Errorf("daddylive: decoding source failed: %w", err)

	}

	streamURL := strings.TrimSpace(string(decoded))

	if !strings.HasPrefix(streamURL, "http") {

		return nil, ErrStreamNotFound

	}

	referer := originOf(playerURL) + "/"

	// A dead channel still resolves to a signed URL, but serves a placeholder instead of a playlist.
	playlist, err := c.fetch(ctx, streamURL, referer)

	if err != nil || !strings.Contains(playlist, "#EXTM3U") {

		slog.Error("daddylive channel offline", "channel", channelID, "player", playerURL)

		return nil, ErrChannelOffline

	}

	slog.Info("daddylive resolved", "channel", channelID, "player", playerURL)

	return &Stream{

		URL: streamURL,
		Referer: referer,

	}, nil

}

func findSource(body string) string {

	if match := sourcePattern.FindStringSubmatch(body); match != nil {

		return match[1]

	}

	if match := atobPattern.FindStringSubmatch(body); match != nil {

		return match[1]

	}

	return ""

}

func (c *Client) fetch(ctx context.Context, target string, referer string) (string, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return "", err

	}

	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.http.Do(req)

	if err != nil {

		return "", fmt.Errorf("daddylive: fetching %s failed: %w", target, err)

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return "", fmt.Errorf("daddylive: %s returned %s", target, resp.Status)

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	if err != nil {

		return "", err

	}

	return string(body), nil

}

func originOf(raw string) string {

	parsed, err := url.Parse(raw)

	if err != nil || parsed.Scheme == "" || parsed.Host == "" {

		return baseURL

	}

	return parsed.Scheme + "://" + parsed.Host

}
