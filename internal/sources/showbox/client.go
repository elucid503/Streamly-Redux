package showbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Rotates without warning, so it lives next to the parser that depends on it (see _docs/DESIGN.md §10).
const (
	apiURL   = "https://mbpapi.shegu.net/api/api_client/index/"
	mediaURL = "https://www.showbox.media"
)

const maxBodyBytes = 8 << 20
const requestAttempts = 3

const (
	BoxTypeMovie  = 1
	BoxTypeSeries = 2
)

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

// The API rejects an already-expired envelope, so this is a real expiry rather than a constant.
const envelopeLifetime = 12 * time.Hour

// Fields every module carries; module-specific arguments are merged on top.
func baseRequest(module string) map[string]any {

	return map[string]any{

		"module":    module,
		"childmode": "0",

		"APP_VERSION": "11.5",
		"LANG":        "en",
		"PLATFORM":    "android",
		"CHANNEL":     "Website",

		"APPID":   "27",
		"VERSION": "129",
		"MEDIUM":  "Website",

		"expired_date": strconv.FormatInt(time.Now().Add(envelopeLifetime).Unix(), 10),
	}

}

func (c *Client) call(ctx context.Context, module string, args map[string]any, out any) error {

	payload := baseRequest(module)

	for key, value := range args {

		payload[key] = value

	}

	data, err := sealRequest(payload)

	if err != nil {

		return fmt.Errorf("showbox: sealing %s failed: %w", module, err)

	}

	form := url.Values{}

	form.Set("appid", "27")
	form.Set("platform", "android")
	form.Set("version", "129")
	form.Set("medium", "Website")
	form.Set("data", data)

	encoded := form.Encode()

	var lastErr error

	for attempt := 0; attempt < requestAttempts; attempt++ {

		lastErr = c.callOnce(ctx, module, encoded, out)

		if lastErr == nil || !transient(lastErr) {

			return lastErr

		}

		if attempt == requestAttempts-1 {

			break

		}

		delay := time.Duration(attempt+1) * 150 * time.Millisecond

		select {

		case <-ctx.Done():

			return ctx.Err()

		case <-time.After(delay):

		}

	}

	return lastErr

}

func (c *Client) callOnce(ctx context.Context, module string, encoded string, out any) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(encoded))

	if err != nil {

		return err

	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := c.do(req)

	if err != nil {

		return fmt.Errorf("showbox: %s failed: %w", module, err)

	}

	var wrapper struct {
		Code    int    `json:"code"`
		Message string `json:"msg"`

		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(body, &wrapper); err != nil {

		return fmt.Errorf("showbox: %s returned unreadable json: %w", module, err)

	}

	// A rejected envelope answers 200 with an empty payload, which would otherwise read as "no results".
	if wrapper.Code != 1 {

		return fmt.Errorf("showbox: %s rejected: %s", module, wrapper.Message)

	}

	if len(wrapper.Data) == 0 || string(wrapper.Data) == "null" {

		return fmt.Errorf("showbox: %s returned no data", module)

	}

	return json.Unmarshal(wrapper.Data, out)

}

func transient(err error) bool {

	if errors.Is(err, io.ErrUnexpectedEOF) {

		return true

	}

	message := err.Error()

	return strings.Contains(message, " returned unreadable json:") ||
		strings.Contains(message, " returned no data") ||
		strings.Contains(message, " failed:")

}

func (c *Client) do(req *http.Request) ([]byte, error) {

	resp, err := c.http.Do(req)

	if err != nil {

		return nil, err

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("%s returned %s", req.URL.Host, resp.Status)

	}

	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

}

// ShareKey is the Febbox folder token for a title; everything about playback hangs off it.
func (c *Client) ShareKey(ctx context.Context, id string, boxType int) (string, error) {

	target := fmt.Sprintf("%s/index/share_link?id=%s&type=%d", mediaURL, url.QueryEscape(id), boxType)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return "", err

	}

	body, err := c.do(req)

	if err != nil {

		return "", fmt.Errorf("showbox: share link failed: %w", err)

	}

	var result struct {
		Data struct {
			Link string `json:"link"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {

		return "", err

	}

	link := strings.TrimSuffix(result.Data.Link, "/")

	if link == "" {

		return "", fmt.Errorf("showbox: no share link for %s", id)

	}

	return link[strings.LastIndex(link, "/")+1:], nil

}
