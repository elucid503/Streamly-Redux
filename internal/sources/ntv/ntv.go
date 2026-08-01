package ntv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

const baseURL = "https://ntv.cx"

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

const maxBodyBytes = 8 << 20

const channelTTL = 10 * time.Minute

var (

	ErrChannelUnknown = errors.New("ntv: channel code not listed")
	ErrNoDirectManifest = errors.New("ntv: channel hides its manifest behind an obfuscated embed")

)

// Only channels whose player page exposes a manifest in plain sight are in scope (see _docs/DESIGN.md §5.1).
var manifestPattern = regexp.MustCompile(`(?i)["'](https?://[^"'\s]+\.m3u8[^"'\s]*)["']`)

type Channel struct {

	ID string
	Name string
	Code string

	URL string

}

type Stream struct {

	URL string
	Referer string

}

type Client struct {

	http *http.Client

	mu sync.Mutex
	channels []Channel
	fetchedAt time.Time

}

func New() *Client {

	httpClient := &http.Client{

		Timeout: 25 * time.Second,

	}

	return &Client{

		http: httpClient,

	}

}

func (c *Client) Channels(ctx context.Context) ([]Channel, error) {

	c.mu.Lock()

	if time.Since(c.fetchedAt) < channelTTL && c.channels != nil {

		cached := c.channels

		c.mu.Unlock()

		return cached, nil

	}

	c.mu.Unlock()

	body, err := c.fetch(ctx, baseURL+"/api/get-channels", baseURL+"/")

	if err != nil {

		return nil, err

	}

	var result struct {

		Success bool `json:"success"`

		Channels []struct {

			ID string `json:"channel_id"`
			Name string `json:"channel_name"`
			Code string `json:"channel_code"`
			URL string `json:"channel_url"`

		} `json:"channels"`

	}

	if err := json.Unmarshal([]byte(body), &result); err != nil {

		return nil, fmt.Errorf("ntv: unreadable channel list: %w", err)

	}

	channels := make([]Channel, 0, len(result.Channels))

	for _, entry := range result.Channels {

		channels = append(channels, Channel{

			ID: entry.ID,
			Name: entry.Name,
			Code: entry.Code,

			URL: entry.URL,

		})

	}

	c.mu.Lock()

	c.channels = channels
	c.fetchedAt = time.Now()

	c.mu.Unlock()

	return channels, nil

}

func (c *Client) Resolve(ctx context.Context, code string) (*Stream, error) {

	channels, err := c.Channels(ctx)

	if err != nil {

		return nil, err

	}

	var player string

	for _, channel := range channels {

		if strings.EqualFold(channel.Code, code) || strings.EqualFold(channel.ID, code) {

			player = channel.URL
			break

		}

	}

	if player == "" {

		return nil, ErrChannelUnknown

	}

	page, err := c.fetch(ctx, player, baseURL+"/")

	if err != nil {

		return nil, err

	}

	match := manifestPattern.FindStringSubmatch(page)

	if match == nil {

		return nil, ErrNoDirectManifest

	}

	return &Stream{

		URL: strings.ReplaceAll(match[1], `\/`, "/"),
		Referer: originOf(player) + "/",

	}, nil

}

func (c *Client) fetch(ctx context.Context, target string, referer string) (string, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return "", err

	}

	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Referer", referer)
	req.Header.Set("Accept", "text/html,application/json,*/*;q=0.8")

	resp, err := c.http.Do(req)

	if err != nil {

		return "", fmt.Errorf("ntv: fetching %s failed: %w", target, err)

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return "", fmt.Errorf("ntv: %s returned %s", target, resp.Status)

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
