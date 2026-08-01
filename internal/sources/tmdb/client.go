package tmdb

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

const (
	apiURL   = "https://api.themoviedb.org/3"
	imageURL = "https://image.tmdb.org/t/p"
)

const maxBodyBytes = 8 << 20

var ErrNotConfigured = errors.New("tmdb: no api key")

type Kind string

const (
	KindMovie Kind = "movie"
	KindTV    Kind = "tv"
)

type Title struct {
	ID   int
	Kind Kind

	Title string
	Year  int

	Poster      string
	Backdrop    string
	Description string
	Rating      string
	Genres      []string
}

type Client struct {
	apiKey string
	http   *http.Client
}

func New(apiKey string) *Client {

	return &Client{

		apiKey: strings.TrimSpace(apiKey),
		http:   &http.Client{Timeout: 15 * time.Second},
	}

}

func (c *Client) Configured() bool {

	return c.apiKey != ""

}

func (c *Client) Trending(ctx context.Context, kind Kind, limit int) ([]Title, error) {

	return c.list(ctx, "/trending/"+string(kind)+"/week", kind, limit)

}

// NowPlaying is distinct from weekly trending — theatrical releases rather than pure social heat.
func (c *Client) NowPlaying(ctx context.Context, limit int) ([]Title, error) {

	return c.list(ctx, "/movie/now_playing", KindMovie, limit)

}

func (c *Client) list(ctx context.Context, path string, kind Kind, limit int) ([]Title, error) {

	var response struct {
		Results []rawTitle `json:"results"`
	}

	if err := c.get(ctx, path, &response); err != nil {

		return nil, err

	}

	if limit <= 0 || limit > len(response.Results) {

		limit = len(response.Results)

	}

	titles := make([]Title, 0, limit)

	for _, raw := range response.Results[:limit] {

		titles = append(titles, raw.title(kind))

	}

	return titles, nil

}

func (c *Client) Details(ctx context.Context, kind Kind, id int) (*Title, error) {

	var raw rawTitle

	if err := c.get(ctx, "/"+string(kind)+"/"+strconv.Itoa(id), &raw); err != nil {

		return nil, err

	}

	title := raw.title(kind)

	return &title, nil

}

type rawTitle struct {
	ID int `json:"id"`

	Title string `json:"title"`
	Name  string `json:"name"`

	ReleaseDate  string `json:"release_date"`
	FirstAirDate string `json:"first_air_date"`

	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path"`
	Overview     string `json:"overview"`

	VoteAverage float64 `json:"vote_average"`
	GenreIDs    []int   `json:"genre_ids"`
}

func (r rawTitle) title(kind Kind) Title {

	title := r.Title
	date := r.ReleaseDate

	if kind == KindTV {

		title = r.Name
		date = r.FirstAirDate

	}

	year := 0

	if len(date) >= 4 {

		year, _ = strconv.Atoi(date[:4])

	}

	rating := ""

	if r.VoteAverage > 0 {

		rating = strconv.FormatFloat(r.VoteAverage, 'f', 1, 64)

	}

	return Title{

		ID:   r.ID,
		Kind: kind,

		Title: title,
		Year:  year,

		Poster:      artwork(r.PosterPath, "w500"),
		Backdrop:    artwork(r.BackdropPath, "w1280"),
		Description: r.Overview,
		Rating:      rating,
		Genres:      genreNames(r.GenreIDs),
	}

}

// TMDB only returns genre ids on list endpoints; names are stable and not worth a second request.
func genreNames(ids []int) []string {

	if len(ids) == 0 {

		return nil

	}

	names := make([]string, 0, len(ids))
	seen := map[string]bool{}

	for _, id := range ids {

		name := genreName(id)

		if name == "" || seen[name] {

			continue

		}

		seen[name] = true
		names = append(names, name)

	}

	return names

}

func genreName(id int) string {

	switch id {

	case 28, 10759:
		return "Action"
	case 12:
		return "Adventure"
	case 16:
		return "Animation"
	case 35:
		return "Comedy"
	case 80:
		return "Crime"
	case 99:
		return "Documentary"
	case 18:
		return "Drama"
	case 10751:
		return "Family"
	case 14, 10765:
		return "Fantasy"
	case 36:
		return "History"
	case 27:
		return "Horror"
	case 10402:
		return "Music"
	case 9648:
		return "Mystery"
	case 10749:
		return "Romance"
	case 878:
		return "Sci-Fi"
	case 10770:
		return "TV Movie"
	case 53:
		return "Thriller"
	case 10752, 10768:
		return "War"
	case 37:
		return "Western"
	case 10762:
		return "Kids"
	case 10763:
		return "News"
	case 10764:
		return "Reality"
	case 10766:
		return "Soap"
	case 10767:
		return "Talk"

	}

	return ""

}

func (c *Client) get(ctx context.Context, path string, out any) error {

	if !c.Configured() {

		return ErrNotConfigured

	}

	params := url.Values{}

	params.Set("api_key", c.apiKey)
	params.Set("language", "en-US")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL+path+"?"+params.Encode(), nil)

	if err != nil {

		return err

	}

	resp, err := c.http.Do(req)

	if err != nil {

		return err

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return fmt.Errorf("tmdb: %s returned %s", path, resp.Status)

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	if err != nil {

		return err

	}

	return json.Unmarshal(body, out)

}

func artwork(path string, size string) string {

	if path == "" {

		return ""

	}

	return imageURL + "/" + size + "/" + strings.TrimPrefix(path, "/")

}
