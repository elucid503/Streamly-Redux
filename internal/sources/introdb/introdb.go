package introdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const baseURL = "https://api.theintrodb.org/v3"

const maxBodyBytes = 1 << 20

type Query struct {

	TmdbID int
	ImdbID string

	Season int
	Episode int

	DurationMs int64

}

type Range struct {

	StartMs int64 `json:"startMs"`
	EndMs int64 `json:"endMs"`

}

type Client struct {

	http *http.Client

	token string

}

func New(token string) *Client {

	httpClient := &http.Client{

		Timeout: 10 * time.Second,

	}

	return &Client{

		http: httpClient,

		token: token,

	}

}

// Autoplaying a season means one lookup per episode, so the bearer token is used rather than the anonymous tier.
func (c *Client) Intro(ctx context.Context, query Query) ([]Range, error) {

	values := url.Values{}

	if query.TmdbID != 0 {

		values.Set("tmdb_id", strconv.Itoa(query.TmdbID))

	}

	if query.ImdbID != "" {

		values.Set("imdb_id", query.ImdbID)

	}

	if query.Season != 0 {

		values.Set("season", strconv.Itoa(query.Season))

	}

	if query.Episode != 0 {

		values.Set("episode", strconv.Itoa(query.Episode))

	}

	if query.DurationMs != 0 {

		values.Set("duration_ms", strconv.FormatInt(query.DurationMs, 10))

	}

	if len(values) == 0 {

		return nil, nil

	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/media?"+values.Encode(), nil)

	if err != nil {

		return nil, err

	}

	req.Header.Set("Accept", "application/json")

	if c.token != "" {

		req.Header.Set("Authorization", "Bearer "+c.token)

	}

	resp, err := c.http.Do(req)

	if err != nil {

		return nil, fmt.Errorf("introdb: request failed: %w", err)

	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {

		return nil, nil

	}

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("introdb: lookup returned %s", resp.Status)

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	if err != nil {

		return nil, err

	}

	var result struct {

		Intro []struct {

			StartMs int64 `json:"start_ms"`
			EndMs int64 `json:"end_ms"`

		} `json:"intro"`

	}

	if err := json.Unmarshal(body, &result); err != nil {

		return nil, fmt.Errorf("introdb: unreadable response: %w", err)

	}

	ranges := make([]Range, 0, len(result.Intro))

	for _, entry := range result.Intro {

		if entry.EndMs <= entry.StartMs {

			continue

		}

		ranges = append(ranges, Range{

			StartMs: entry.StartMs,
			EndMs: entry.EndMs,

		})

	}

	return ranges, nil

}
