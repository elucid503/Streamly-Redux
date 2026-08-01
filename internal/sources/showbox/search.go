package showbox

import (
	"context"
	"strings"
)

type Title struct {
	ID      string
	BoxType int

	Title string
	Year  int

	Poster      string
	Description string
	Rating      string
}

// Showbox returns ids and years as numbers in some modules and quoted strings in others.
type flexText string

func (f *flexText) UnmarshalJSON(data []byte) error {

	*f = flexText(strings.Trim(string(data), `"`))

	return nil

}

type rawTitle struct {
	ID      flexText `json:"id"`
	BoxType int      `json:"box_type"`

	Title string `json:"title"`
	Year  int    `json:"year"`

	Poster      string `json:"poster"`
	Description string `json:"description"`
	Rating      string `json:"imdb_rating"`
}

func (r rawTitle) title() Title {

	return Title{

		ID:      string(r.ID),
		BoxType: r.BoxType,

		Title: r.Title,
		Year:  r.Year,

		Poster:      r.Poster,
		Description: r.Description,
		Rating:      r.Rating,
	}

}

func (c *Client) Search(ctx context.Context, keyword string, limit int) ([]Title, error) {

	var raw []rawTitle

	args := map[string]any{

		"keyword": keyword,
		"type":    "all",

		"page":      1,
		"pagelimit": limit,
	}

	if err := c.call(ctx, "Search5", args, &raw); err != nil {

		return nil, err

	}

	titles := make([]Title, 0, len(raw))

	for _, entry := range raw {

		titles = append(titles, entry.title())

	}

	return titles, nil

}
