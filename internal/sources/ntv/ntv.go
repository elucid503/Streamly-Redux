package ntv

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Mirrors Streamly-Web media/internal/live/source/ntv.go: catalog from ntv.cx,
// stream URL assembled from the cdnlive player page, playlist verified before return.

const baseURL = "https://ntv.cx"

const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

const maxBodyBytes = 8 << 20

const channelTTL = 30 * time.Minute

const cdnReferer = "https://cdnlivetv.tv/"

var (

	ErrChannelUnknown = errors.New("ntv: channel not listed")
	ErrNoStream = errors.New("ntv: player page has no stream url")
	ErrPlaylistUnplayable = errors.New("ntv: playlist is not playable")

)

var (

	varDeclPattern = regexp.MustCompile(`var\s+(\w+)\s*=\s*'([^']*)';`)
	chainPattern = regexp.MustCompile(`var\s+\w+\s*=\s*((?:\w+\(\w+\)\s*\+\s*)*\w+\(\w+\))\s*;`)
	callPattern = regexp.MustCompile(`\w+\((\w+)\)`)

)

type Channel struct {

	ID string
	Name string
	Code string

	URL string
	Server string

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

	body, err := c.fetch(ctx, baseURL+"/api/get-channels", map[string]string{
		"Accept": "application/json",
		"Accept-Language": "en-US,en;q=0.9",
	})

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
			Server string `json:"server"`

		} `json:"channels"`

	}

	if err := json.Unmarshal([]byte(body), &result); err != nil {

		return nil, fmt.Errorf("ntv: unreadable channel list: %w", err)

	}

	if !result.Success {

		return nil, fmt.Errorf("ntv: channels response reported failure")

	}

	// Prefer cdnlive — historically the only ntv server that resolved cleanly.
	filtered := make([]Channel, 0, len(result.Channels))

	for _, entry := range result.Channels {

		if entry.Server != "" && entry.Server != "cdnlive" {

			continue

		}

		filtered = append(filtered, Channel{

			ID: entry.ID,
			Name: entry.Name,
			Code: entry.Code,

			URL: entry.URL,
			Server: entry.Server,

		})

	}

	if len(filtered) == 0 {

		for _, entry := range result.Channels {

			filtered = append(filtered, Channel{

				ID: entry.ID,
				Name: entry.Name,
				Code: entry.Code,

				URL: entry.URL,
				Server: entry.Server,

			})

		}

	}

	c.mu.Lock()

	c.channels = filtered
	c.fetchedAt = time.Now()

	c.mu.Unlock()

	return filtered, nil

}

// Resolve looks up a channel by id (preferred catalog ref) or name.
func (c *Client) Resolve(ctx context.Context, ref string) (*Stream, error) {

	ref = strings.TrimSpace(ref)

	if ref == "" {

		return nil, ErrChannelUnknown

	}

	channels, err := c.Channels(ctx)

	if err != nil {

		return nil, err

	}

	var player string

	for _, channel := range channels {

		if strings.EqualFold(channel.ID, ref) || strings.EqualFold(channel.Name, ref) {

			player = channel.URL
			break

		}

	}

	if player == "" {

		return nil, ErrChannelUnknown

	}

	page, err := c.fetch(ctx, player, map[string]string{
		"Accept-Language": "en-US,en;q=0.9",
	})

	if err != nil {

		return nil, err

	}

	streamURL, err := extractStreamURL(page)

	if err != nil {

		return nil, err

	}

	headers := map[string]string{

		"Referer": cdnReferer,
		"Origin": "https://cdnlivetv.tv",
		"User-Agent": browserUA,

	}

	if !c.playlistPlayable(ctx, streamURL, headers) {

		// Some edges accept bare requests without referer.
		if !c.playlistPlayable(ctx, streamURL, map[string]string{"User-Agent": browserUA}) {

			return nil, ErrPlaylistUnplayable

		}

		return &Stream{

			URL: streamURL,
			Referer: "",

		}, nil

	}

	return &Stream{

		URL: streamURL,
		Referer: cdnReferer,

	}, nil

}

func extractStreamURL(html string) (string, error) {

	literals := make(map[string]string)

	for _, match := range varDeclPattern.FindAllStringSubmatch(html, -1) {

		literals[match[1]] = match[2]

	}

	if len(literals) == 0 {

		return "", ErrNoStream

	}

	var best []string

	for _, chain := range chainPattern.FindAllStringSubmatch(html, -1) {

		names := make([]string, 0)
		ok := true

		for _, call := range callPattern.FindAllStringSubmatch(chain[1], -1) {

			if _, exists := literals[call[1]]; !exists {

				ok = false
				break

			}

			names = append(names, call[1])

		}

		if ok && len(names) > len(best) {

			best = names

		}

	}

	if len(best) == 0 {

		return "", ErrNoStream

	}

	var builder strings.Builder

	for _, name := range best {

		decoded, err := decodeURLSafeBase64(literals[name])

		if err != nil {

			return "", fmt.Errorf("ntv: decode fragment: %w", err)

		}

		builder.Write(decoded)

	}

	streamURL := builder.String()

	if !strings.HasPrefix(streamURL, "http://") && !strings.HasPrefix(streamURL, "https://") {

		return "", fmt.Errorf("ntv: decoded stream url invalid")

	}

	return streamURL, nil

}

func decodeURLSafeBase64(value string) ([]byte, error) {

	value = strings.ReplaceAll(value, "-", "+")
	value = strings.ReplaceAll(value, "_", "/")

	for len(value)%4 != 0 {

		value += "="

	}

	return base64.StdEncoding.DecodeString(value)

}

func (c *Client) playlistPlayable(ctx context.Context, target string, headers map[string]string) bool {

	if target == "" {

		return false

	}

	body, err := c.fetch(ctx, target, headers)

	if err != nil {

		return false

	}

	return strings.Contains(body, "#EXTM3U")

}

func (c *Client) fetch(ctx context.Context, target string, headers map[string]string) (string, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return "", err

	}

	req.Header.Set("User-Agent", browserUA)
	req.Header.Set("Accept", "*/*")

	for key, value := range headers {

		if value != "" {

			req.Header.Set(key, value)

		}

	}

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
