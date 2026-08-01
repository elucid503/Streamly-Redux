package subdl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	apiURL      = "https://api.subdl.com/api/v1/subtitles"
	downloadURL = "https://dl.subdl.com"
)

const maxBodyBytes = 8 << 20

var (
	ErrNotConfigured = errors.New("subdl: api key not configured")

	// Searching is effectively unmetered while downloading is not, so exhaustion must not read as a generic failure (§5.5).
	ErrDownloadBudget = errors.New("subdl: daily download allowance exhausted")
)

type Query struct {
	ImdbID string
	TmdbID int

	Series bool

	Season  int
	Episode int

	ReleaseName string
}

type Release struct {
	Path string
	Name string

	ReleaseName string
	Language    string

	HearingImpaired bool
}

type Client struct {
	http *http.Client

	apiKey string
	cache  *cache
}

func New(apiKey string) *Client {

	httpClient := &http.Client{

		Timeout: 20 * time.Second,
	}

	return &Client{

		http: httpClient,

		apiKey: apiKey,
		cache:  newCache(),
	}

}

func (c *Client) Configured() bool { return c.apiKey != "" }

// Searching is effectively unmetered; downloading is not, so this returns candidates and defers fetching (see _docs/DESIGN.md §5.5).
func (c *Client) Search(ctx context.Context, query Query) ([]Release, error) {

	if c.apiKey == "" {

		return nil, ErrNotConfigured

	}

	values := url.Values{}

	values.Set("api_key", c.apiKey)
	values.Set("languages", "EN,ES,FR,DE,PT,IT,JA,KO,ZH,RU,AR,HI,NL,PL,TR")
	values.Set("unpack", "1")

	if query.Series {

		values.Set("type", "tv")

	} else {

		values.Set("type", "movie")

	}

	if query.ImdbID != "" {

		values.Set("imdb_id", query.ImdbID)

	}

	if query.TmdbID != 0 {

		values.Set("tmdb_id", strconv.Itoa(query.TmdbID))

	}

	if query.Season != 0 {

		values.Set("season_number", strconv.Itoa(query.Season))

	}

	if query.Episode != 0 {

		values.Set("episode_number", strconv.Itoa(query.Episode))

	}

	body, err := c.get(ctx, apiURL+"?"+values.Encode())

	if err != nil {

		return nil, err

	}

	var result struct {
		Status bool   `json:"status"`
		Error  string `json:"error"`

		Subtitles []struct {
			ReleaseName string `json:"release_name"`
			Name        string `json:"name"`
			URL         string `json:"url"`

			Season  int `json:"season"`
			Episode int `json:"episode"`

			HI bool `json:"hi"`

			UnpackFiles []struct {
				URL  string `json:"url"`
				Name string `json:"name"`

				ReleaseName string `json:"release_name"`
				Language    string `json:"language"`

				HI bool `json:"hi"`
			} `json:"unpack_files"`
		} `json:"subtitles"`
	}

	if err := json.Unmarshal(body, &result); err != nil {

		return nil, fmt.Errorf("subdl: unreadable response: %w", err)

	}

	if !result.Status && result.Error != "" {

		return nil, fmt.Errorf("subdl: %s", result.Error)

	}

	releases := make([]Release, 0, len(result.Subtitles))

	for _, entry := range result.Subtitles {

		if len(entry.UnpackFiles) > 0 {

			for _, file := range entry.UnpackFiles {

				releases = append(releases, Release{

					Path: file.URL,
					Name: file.Name,

					ReleaseName: pick(file.ReleaseName, entry.ReleaseName),
					Language:    file.Language,

					HearingImpaired: file.HI,
				})

			}

			continue

		}

		if entry.URL == "" {

			continue

		}

		releases = append(releases, Release{

			Path: entry.URL,
			Name: pick(entry.Name, entry.ReleaseName),

			ReleaseName: entry.ReleaseName,
			Language:    "EN",

			HearingImpaired: entry.HI,
		})

	}

	rank(releases, query.ReleaseName)

	return releases, nil

}

// Converted tracks are cached because the free tier allows far fewer downloads than searches.
func (c *Client) Track(ctx context.Context, path string) ([]byte, error) {

	if cached, ok := c.cache.get(path); ok {

		return cached, nil

	}

	target := path

	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {

		target = downloadURL + "/" + strings.TrimPrefix(path, "/")

	}

	raw, err := c.get(ctx, target)

	if err != nil {

		return nil, err

	}

	vtt, err := toWebVTT(raw)

	if err != nil {

		return nil, err

	}

	c.cache.put(path, vtt)

	return vtt, nil

}

func (c *Client) get(ctx context.Context, target string) ([]byte, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return nil, err

	}

	req.Header.Set("Accept", "*/*")

	resp, err := c.http.Do(req)

	if err != nil {

		return nil, fmt.Errorf("subdl: request failed: %w", err)

	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {

		return nil, ErrDownloadBudget

	}

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("subdl: %s returned %s", target, resp.Status)

	}

	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

}

// A wrong-release match drifts out of time, so the closest name is offered first and the rest stay available.
func rank(releases []Release, releaseName string) {

	if releaseName == "" {

		return

	}

	wanted := tokenise(releaseName)

	sort.SliceStable(releases, func(a int, b int) bool {

		return overlap(wanted, tokenise(releases[a].ReleaseName)) > overlap(wanted, tokenise(releases[b].ReleaseName))

	})

}

func tokenise(name string) map[string]bool {

	tokens := map[string]bool{}

	for _, token := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {

		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9')

	}) {

		tokens[token] = true

	}

	return tokens

}

func overlap(wanted map[string]bool, candidate map[string]bool) int {

	score := 0

	for token := range candidate {

		if wanted[token] {

			score++

		}

	}

	return score

}

func pick(primary string, fallback string) string {

	if primary != "" {

		return primary

	}

	return fallback

}
