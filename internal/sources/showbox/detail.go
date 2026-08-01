package showbox

import (
	"context"
	"sort"
)

type Episode struct {
	Season  int
	Episode int

	Title string
	Thumbnail string
	Description string
}

type Detail struct {
	ID      string
	BoxType int

	Title string
	Year  string

	Poster      string
	Banner      string
	Description string
	Rating      string

	TmdbID int
	ImdbID string

	Episodes []Episode
}

type rawDetail struct {
	Title string   `json:"title"`
	Year  flexText `json:"year"`

	Poster      string `json:"poster"`
	PosterOrg   string `json:"poster_org"`
	Banner      string `json:"banner"`
	Backdrop    string `json:"backdrop"`
	Cover       string `json:"cover"`
	Still       string `json:"still"`
	Description string `json:"description"`
	Rating      string `json:"imdb_rating"`

	TmdbID int    `json:"tmdb_id"`
	ImdbID string `json:"imdb_id"`

	Episode []struct {
		Season  int `json:"season"`
		Episode int `json:"episode"`

		Title string `json:"title"`
	} `json:"episode"`
}

func (c *Client) MovieDetail(ctx context.Context, id string) (*Detail, error) {

	var raw rawDetail

	if err := c.call(ctx, "Movie_detail", map[string]any{"mid": id}, &raw); err != nil {

		return nil, err

	}

	detail := raw.detail(id, BoxTypeMovie)

	return detail, nil

}

func (c *Client) SeriesDetail(ctx context.Context, id string) (*Detail, error) {

	var raw rawDetail

	if err := c.call(ctx, "TV_detail_v2", map[string]any{"tid": id}, &raw); err != nil {

		return nil, err

	}

	return raw.detail(id, BoxTypeSeries), nil

}

func (r rawDetail) detail(id string, boxType int) *Detail {

	poster := r.PosterOrg

	if poster == "" {

		poster = r.Poster

	}

	banner := first(r.Banner, r.Backdrop, r.Cover, r.Still)

	episodes := make([]Episode, 0, len(r.Episode))

	for _, entry := range r.Episode {

		episodes = append(episodes, Episode{

			Season:  entry.Season,
			Episode: entry.Episode,

			Title: entry.Title,
		})

	}

	sort.Slice(episodes, func(a int, b int) bool {

		if episodes[a].Season != episodes[b].Season {

			return episodes[a].Season < episodes[b].Season

		}

		return episodes[a].Episode < episodes[b].Episode

	})

	return &Detail{

		ID:      id,
		BoxType: boxType,

		Title: r.Title,
		Year:  string(r.Year),

		Poster:      poster,
		Banner:      banner,
		Description: r.Description,
		Rating:      r.Rating,

		TmdbID: r.TmdbID,
		ImdbID: r.ImdbID,

		Episodes: episodes,
	}

}

func first(values ...string) string {

	for _, value := range values {

		if value != "" {

			return value

		}

	}

	return ""

}
