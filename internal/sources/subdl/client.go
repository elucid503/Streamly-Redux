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
	apiURL = "https://api.subdl.com/api/v1/subtitles"
	downloadURL = "https://dl.subdl.com"
	userAgent = "Streamly/1.0"
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

	Season int
	Episode int

	// ReleaseName is sent as SubDL's file_name filter when present (Streamly-Web VideoName).
	ReleaseName string

}

type Release struct {

	Path string
	Name string

	ReleaseName string
	Language string
	Format string

	HearingImpaired bool

}

type Client struct {

	http *http.Client

	apiKey string
	cache *cache

}

func New(apiKey string) *Client {

	return &Client{

		http: &http.Client{Timeout: 20 * time.Second},

		apiKey: strings.TrimSpace(apiKey),
		cache: newCache(),

	}

}

func (c *Client) Configured() bool {

	return c.apiKey != ""

}

// Search returns ranked English candidates. Downloads stay deferred until Track is called (§5.5).
func (c *Client) Search(ctx context.Context, query Query) ([]Release, error) {

	if c.apiKey == "" {

		return nil, ErrNotConfigured

	}

	var releases []Release
	var firstErr error
	seen := make(map[string]struct{})

	for _, candidate := range queryVariants(query) {

		entries, err := c.search(ctx, candidate)

		if err != nil {

			if firstErr == nil {

				firstErr = err

			}

			continue

		}

		for _, release := range pickTracks(entries, candidate.Season, candidate.Episode) {

			key := strings.TrimSpace(release.Path)

			if key == "" {

				continue

			}

			if _, ok := seen[key]; ok {

				continue

			}

			seen[key] = struct{}{}
			releases = append(releases, release)

		}

	}

	if len(releases) == 0 {

		if firstErr != nil {

			return nil, firstErr

		}

		return []Release{}, nil

	}

	rank(releases, query.ReleaseName)

	return releases, nil

}

// Track downloads a candidate and converts it to WebVTT. Season/episode guide zip member selection.
func (c *Client) Track(ctx context.Context, path string, season, episode int) ([]byte, error) {

	cacheKey := path

	if season > 0 || episode > 0 {

		cacheKey = fmt.Sprintf("%s|%d|%d", path, season, episode)

	}

	if cached, ok := c.cache.get(cacheKey); ok {

		return cached, nil

	}

	raw, err := c.downloadBytes(ctx, path)

	if err != nil {

		return nil, err

	}

	vtt, err := toWebVTT(raw, season, episode)

	if err != nil {

		return nil, err

	}

	c.cache.put(cacheKey, vtt)

	return vtt, nil

}

func queryVariants(query Query) []Query {

	var variants []Query
	seen := make(map[string]struct{})

	add := func(candidate Query) {

		if strings.TrimSpace(candidate.ImdbID) == "" && candidate.TmdbID <= 0 {

			return

		}

		key := strings.Join([]string{
			strings.TrimSpace(candidate.ImdbID),
			strconv.Itoa(candidate.TmdbID),
			strings.TrimSpace(candidate.ReleaseName),
			strconv.Itoa(candidate.Season),
			strconv.Itoa(candidate.Episode),
			strconv.FormatBool(candidate.Series),
		}, "\x00")

		if _, ok := seen[key]; ok {

			return

		}

		seen[key] = struct{}{}
		variants = append(variants, candidate)

	}

	add(query)

	withoutName := query
	withoutName.ReleaseName = ""
	add(withoutName)

	if query.ImdbID != "" && query.TmdbID > 0 {

		tmdbQuery := query
		tmdbQuery.ImdbID = ""
		add(tmdbQuery)

		tmdbWithoutName := tmdbQuery
		tmdbWithoutName.ReleaseName = ""
		add(tmdbWithoutName)

	}

	return variants

}

func (c *Client) search(ctx context.Context, query Query) ([]subtitleEntry, error) {

	params := url.Values{}

	params.Set("api_key", c.apiKey)
	// English only — multi-language lists were noisy and poorly matched (Streamly-Web).
	params.Set("languages", "EN")
	params.Set("unpack", "1")

	if query.Series || (query.Season > 0 && query.Episode > 0) {

		params.Set("type", "tv")

		if query.Season > 0 {

			params.Set("season_number", strconv.Itoa(query.Season))

		}

		if query.Episode > 0 {

			params.Set("episode_number", strconv.Itoa(query.Episode))

		}

	} else {

		params.Set("type", "movie")

	}

	if imdb := subdlIMDBID(query.ImdbID); imdb != "" {

		params.Set("imdb_id", imdb)

	} else if query.TmdbID > 0 {

		params.Set("tmdb_id", strconv.Itoa(query.TmdbID))

	} else {

		return nil, errors.New("subdl: imdb or tmdb id required")

	}

	if name := strings.TrimSpace(query.ReleaseName); name != "" {

		params.Set("file_name", name)

	}

	body, err := c.get(ctx, apiURL+"?"+params.Encode())

	if err != nil {

		return nil, err

	}

	var result struct {

		Status bool `json:"status"`
		Error string `json:"error"`

		Subtitles []subtitleEntry `json:"subtitles"`

	}

	if err := json.Unmarshal(body, &result); err != nil {

		return nil, fmt.Errorf("subdl: unreadable response: %w", err)

	}

	if !result.Status {

		if strings.TrimSpace(result.Error) != "" {

			return nil, fmt.Errorf("subdl: %s", strings.TrimSpace(result.Error))

		}

		return nil, nil

	}

	return result.Subtitles, nil

}

func (c *Client) downloadBytes(ctx context.Context, path string) ([]byte, error) {

	path = strings.TrimSpace(path)

	data, err := c.downloadURLBytes(ctx, path)

	if err == nil {

		return data, nil

	}

	if withoutQuery, _, ok := strings.Cut(path, "?"); ok && strings.TrimSpace(withoutQuery) != "" {

		if fallback, fallbackErr := c.downloadURLBytes(ctx, withoutQuery); fallbackErr == nil {

			return fallback, nil

		}

	}

	return nil, err

}

func (c *Client) downloadURLBytes(ctx context.Context, path string) ([]byte, error) {

	target := path

	if !strings.HasPrefix(path, "http://") && !strings.HasPrefix(path, "https://") {

		if !strings.HasPrefix(path, "/") {

			path = "/" + path

		}

		target = downloadURL + path

	}

	return c.get(ctx, target)

}

func (c *Client) get(ctx context.Context, target string) ([]byte, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return nil, err

	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")

	resp, err := c.http.Do(req)

	if err != nil {

		return nil, fmt.Errorf("subdl: request failed: %w", err)

	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {

		return nil, ErrDownloadBudget

	}

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {

		return nil, fmt.Errorf("subdl: unauthorized (%s)", resp.Status)

	}

	if resp.StatusCode == http.StatusNotFound {

		return nil, fmt.Errorf("subdl: not found")

	}

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("subdl: %s returned %s", target, resp.Status)

	}

	return io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

}

func subdlIMDBID(id string) string {

	id = strings.TrimSpace(id)

	if id == "" {

		return ""

	}

	if !strings.HasPrefix(strings.ToLower(id), "tt") {

		return "tt" + id

	}

	return id

}

// A wrong-release match drifts out of time, so the closest name is offered first and the rest stay available.
func rank(releases []Release, releaseName string) {

	if releaseName == "" {

		return

	}

	wanted := tokenise(releaseName)

	sort.SliceStable(releases, func(a int, b int) bool {

		return overlap(wanted, tokenise(releases[a].ReleaseName+" "+releases[a].Name)) > overlap(wanted, tokenise(releases[b].ReleaseName+" "+releases[b].Name))

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
