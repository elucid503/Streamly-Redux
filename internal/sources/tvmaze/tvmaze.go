package tvmaze

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://api.tvmaze.com"

const maxBodyBytes = 4 << 20

type Episode struct {
	Season  int
	Episode int

	Title string
	Image string
	Summary string
}

type Client struct {
	http *http.Client
}

func New() *Client {

	httpClient := &http.Client{

		Timeout: 15 * time.Second,
	}

	return &Client{

		http: httpClient,
	}

}

// Secondary episode source for series whose Showbox listing is incomplete (see _docs/DESIGN.md §5.4).
func (c *Client) Episodes(ctx context.Context, imdbID string) ([]Episode, error) {

	var show struct {
		ID int `json:"id"`
	}

	if err := c.get(ctx, fmt.Sprintf("%s/lookup/shows?imdb=%s", baseURL, url.QueryEscape(imdbID)), &show); err != nil {

		return nil, err

	}

	if show.ID == 0 {

		return nil, fmt.Errorf("tvmaze: no show for %s", imdbID)

	}

	var raw []struct {
		Season int `json:"season"`
		Number int `json:"number"`

		Name string `json:"name"`
		Summary string `json:"summary"`

		Image *struct {
			Medium string `json:"medium"`
			Original string `json:"original"`
		} `json:"image"`
	}

	if err := c.get(ctx, fmt.Sprintf("%s/shows/%d/episodes", baseURL, show.ID), &raw); err != nil {

		return nil, err

	}

	episodes := make([]Episode, 0, len(raw))

	for _, entry := range raw {

		if entry.Season == 0 || entry.Number == 0 {

			continue

		}

		image := ""

		if entry.Image != nil {

			image = entry.Image.Medium

			if image == "" {

				image = entry.Image.Original

			}

		}

		episodes = append(episodes, Episode{

			Season: entry.Season,
			Episode: entry.Number,

			Title: entry.Name,
			Image: image,
			Summary: stripTags(entry.Summary),
		})

	}

	return episodes, nil

}

func stripTags(value string) string {

	if value == "" {

		return ""

	}

	var out strings.Builder

	inTag := false

	for _, r := range value {

		switch {

		case r == '<':

			inTag = true

		case r == '>':

			inTag = false

		case !inTag:

			out.WriteRune(r)

		}

	}

	return strings.TrimSpace(html.UnescapeString(out.String()))

}

func (c *Client) get(ctx context.Context, target string, out any) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return err

	}

	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)

	if err != nil {

		return fmt.Errorf("tvmaze: request failed: %w", err)

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return fmt.Errorf("tvmaze: %s returned %s", target, resp.Status)

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	if err != nil {

		return err

	}

	return json.Unmarshal(body, out)

}
