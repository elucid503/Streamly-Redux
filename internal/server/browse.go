package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"streamly/internal/proxy"
	"streamly/internal/sources/showbox"
	"streamly/internal/sources/tmdb"

	"github.com/gin-gonic/gin"
)

const searchLimit = 24
const topPickLimit = 20
const topPickTTL = 30 * time.Minute

type titleView struct {
	ID      string `json:"id"`
	BoxType int    `json:"boxType"`
	Source  string `json:"source,omitempty"`
	TMDBID  int    `json:"tmdbId,omitempty"`

	Title string `json:"title"`
	Year  int    `json:"year"`

	Poster      string   `json:"poster,omitempty"`
	Description string   `json:"description,omitempty"`
	Rating      string   `json:"rating,omitempty"`
	Genres      []string `json:"genres,omitempty"`
}

type episodeView struct {
	Episode int `json:"episode"`
	Title string `json:"title,omitempty"`
	Thumbnail string `json:"thumbnail,omitempty"`
	Description string `json:"description,omitempty"`
}

type seasonView struct {
	Season   int           `json:"season"`
	Episodes []episodeView `json:"episodes"`
}

// One field spans channels and VOD; someone who knows what they want rarely cares which source it lives on (see _docs/DESIGN.md §6.1).
func (a *api) search(c *gin.Context) {

	query := strings.TrimSpace(c.Query("q"))

	if query == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return

	}

	channels := a.catalog.Channels()

	matched := channels[:0:0]

	for _, channel := range channels {

		if strings.Contains(strings.ToLower(channel.Name), strings.ToLower(query)) {

			matched = append(matched, channel)

		}

	}

	titles := []titleView{}

	found, err := a.showbox.Search(c.Request.Context(), query, searchLimit)

	if err != nil {

		slog.Error("showbox search failed", "query", query, "err", err)

	} else {

		titles = titleViews(found)

	}

	c.JSON(http.StatusOK, gin.H{

		"channels": channelViews(matched),
		"titles":   titles,
	})

}

func (a *api) trending(c *gin.Context) {

	if !a.tmdb.Configured() {

		c.JSON(http.StatusOK, gin.H{"movies": []titleView{}, "series": []titleView{}, "nowPlaying": []titleView{}})
		return

	}

	movies, movieErr := a.topPicks(c.Request.Context(), "trending-movie", func(ctx context.Context) ([]tmdb.Title, error) {

		return a.tmdb.Trending(ctx, tmdb.KindMovie, topPickLimit)

	})

	series, seriesErr := a.topPicks(c.Request.Context(), "trending-tv", func(ctx context.Context) ([]tmdb.Title, error) {

		return a.tmdb.Trending(ctx, tmdb.KindTV, topPickLimit)

	})

	if movieErr != nil {

		slog.Error("tmdb movie picks failed", "err", movieErr)

	}

	if seriesErr != nil {

		slog.Error("tmdb series picks failed", "err", seriesErr)

	}

	nowPlaying, nowPlayingErr := a.topPicks(c.Request.Context(), "now-playing", func(ctx context.Context) ([]tmdb.Title, error) {

		return a.tmdb.NowPlaying(ctx, topPickLimit)

	})

	if nowPlayingErr != nil {

		slog.Error("tmdb now playing picks failed", "err", nowPlayingErr)

	}

	if movies == nil {

		movies = []titleView{}

	}

	if series == nil {

		series = []titleView{}

	}

	if nowPlaying == nil {

		nowPlaying = []titleView{}

	}

	c.JSON(http.StatusOK, gin.H{

		"movies": movies,
		"series": series,
		"nowPlaying": nowPlaying,
	})

}

func (a *api) topPicks(ctx context.Context, cacheKey string, fetch func(context.Context) ([]tmdb.Title, error)) ([]titleView, error) {

	a.picksMu.Lock()
	defer a.picksMu.Unlock()

	if cached, ok := a.picks[cacheKey]; ok && time.Now().Before(cached.expires) {

		return append([]titleView(nil), cached.titles...), nil

	}

	titles, err := fetch(ctx)

	if err != nil {

		if a.picks == nil {

			a.picks = map[string]pickCache{}

		}

		a.picks[cacheKey] = pickCache{expires: time.Now().Add(15 * time.Second)}

		return nil, err

	}

	picks := curatedViews(titles)

	if a.picks == nil {

		a.picks = map[string]pickCache{}

	}

	a.picks[cacheKey] = pickCache{

		titles: append([]titleView(nil), picks...),
		expires: time.Now().Add(topPickTTL),
	}

	return picks, nil

}

func (a *api) title(c *gin.Context) {

	boxType, err := strconv.Atoi(c.Param("boxType"))

	if err != nil {

		c.JSON(http.StatusBadRequest, gin.H{"error": "boxType must be 1 or 2"})
		return

	}

	id := c.Param("id")
	ctx := c.Request.Context()

	var detail *showbox.Detail
	var curated *tmdb.Title

	if c.Query("source") == "tmdb" {

		tmdbID, parseErr := strconv.Atoi(id)

		if parseErr != nil {

			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tmdb id"})
			return

		}

		kind := tmdb.KindMovie

		if boxType == showbox.BoxTypeSeries {

			kind = tmdb.KindTV

		}

		curated, err = a.tmdb.Details(ctx, kind, tmdbID)

		if err == nil {

			detail, err = a.resolveCurated(ctx, curated, boxType)

		}

	} else if boxType == showbox.BoxTypeSeries {

		detail, err = a.showbox.SeriesDetail(ctx, id)

	} else {

		detail, err = a.showbox.MovieDetail(ctx, id)

	}

	if err != nil {

		slog.Error("title detail failed", "id", id, "boxType", boxType, "source", c.Query("source"), "err", err)

		c.JSON(http.StatusBadGateway, gin.H{"error": "could not load this title"})
		return

	}

	if curated == nil && detail.TmdbID > 0 && a.tmdb.Configured() {

		kind := tmdb.KindMovie

		if boxType == showbox.BoxTypeSeries {

			kind = tmdb.KindTV

		}

		curated, _ = a.tmdb.Details(ctx, kind, detail.TmdbID)

	}

	poster := detail.Poster
	banner := detail.Banner
	description := detail.Description
	rating := detail.Rating
	titleYear := year(detail.Year)

	if curated != nil {

		poster = prefer(curated.Poster, poster)
		banner = prefer(curated.Backdrop, banner)
		description = prefer(curated.Description, description)
		rating = prefer(curated.Rating, rating)

		if curated.Year > 0 {

			titleYear = curated.Year

		}

	}

	tmdbID := detail.TmdbID

	if curated != nil {

		tmdbID = curated.ID

	}

	seasons := []seasonView{}

	if boxType == showbox.BoxTypeSeries {

		seasons = groupSeasons(a.episodes(c.Request.Context(), detail))

	}

	c.JSON(http.StatusOK, gin.H{

		"id":      detail.ID,
		"boxType": detail.BoxType,

		"title": detail.Title,
		"year":  titleYear,

		"poster":      proxy.ImageURL(poster),
		"banner":      proxy.ImageURL(banner),
		"description": description,
		"rating":      rating,

		"imdbId": detail.ImdbID,
		"tmdbId": tmdbID,

		"seasons": seasons,
	})

}

func (a *api) resolveCurated(ctx context.Context, curated *tmdb.Title, boxType int) (*showbox.Detail, error) {

	found, err := a.showbox.Search(ctx, curated.Title, 16)

	if err != nil {

		return nil, err

	}

	best := ""
	bestScore := -1

	for _, title := range found {

		if title.BoxType != boxType || normalizedTitle(title.Title) != normalizedTitle(curated.Title) {

			continue

		}

		score := 2

		if curated.Year > 0 && title.Year == curated.Year {

			score++

		}

		if score > bestScore {

			best = title.ID
			bestScore = score

		}

	}

	if best == "" {

		return nil, fmt.Errorf("showbox: no streamable match for %q", curated.Title)

	}

	if boxType == showbox.BoxTypeSeries {

		return a.showbox.SeriesDetail(ctx, best)

	}

	return a.showbox.MovieDetail(ctx, best)

}

// TVMaze fills missing episodes and supplies stills when Showbox only has titles (§5.4).
func (a *api) episodes(ctx context.Context, detail *showbox.Detail) []showbox.Episode {

	episodes := detail.Episodes

	if detail.ImdbID == "" {

		return episodes

	}

	secondary, err := a.tvmaze.Episodes(ctx, detail.ImdbID)

	if err != nil {

		slog.Debug("tvmaze lookup failed", "imdb", detail.ImdbID, "err", err)

		return episodes

	}

	titles := map[string]string{}
	thumbs := map[string]string{}
	summaries := map[string]string{}

	for _, episode := range episodes {

		k := key(episode.Season, episode.Episode)

		if episode.Title != "" {

			titles[k] = episode.Title

		}

		if episode.Thumbnail != "" {

			thumbs[k] = episode.Thumbnail

		}

		if episode.Description != "" {

			summaries[k] = episode.Description

		}

	}

	for _, episode := range secondary {

		k := key(episode.Season, episode.Episode)

		if episode.Title != "" && titles[k] == "" {

			titles[k] = episode.Title

		}

		if episode.Image != "" {

			thumbs[k] = episode.Image

		}

		if episode.Summary != "" {

			summaries[k] = episode.Summary

		}

	}

	if len(secondary) > len(episodes) {

		merged := make([]showbox.Episode, 0, len(secondary))

		for _, episode := range secondary {

			k := key(episode.Season, episode.Episode)

			merged = append(merged, showbox.Episode{

				Season: episode.Season,
				Episode: episode.Episode,

				Title: titles[k],
				Thumbnail: thumbs[k],
				Description: summaries[k],
			})

		}

		return merged

	}

	for index := range episodes {

		k := key(episodes[index].Season, episodes[index].Episode)

		if episodes[index].Title == "" {

			episodes[index].Title = titles[k]

		}

		episodes[index].Thumbnail = thumbs[k]
		episodes[index].Description = summaries[k]

	}

	return episodes

}

func groupSeasons(episodes []showbox.Episode) []seasonView {

	bySeason := map[int][]episodeView{}

	for _, episode := range episodes {

		bySeason[episode.Season] = append(bySeason[episode.Season], episodeView{

			Episode: episode.Episode,
			Title: episode.Title,
			Thumbnail: proxy.ImageURL(episode.Thumbnail),
			Description: episode.Description,
		})

	}

	seasons := make([]seasonView, 0, len(bySeason))

	for number, list := range bySeason {

		sort.Slice(list, func(a int, b int) bool { return list[a].Episode < list[b].Episode })

		seasons = append(seasons, seasonView{Season: number, Episodes: list})

	}

	sort.Slice(seasons, func(a int, b int) bool { return seasons[a].Season < seasons[b].Season })

	return seasons

}

func titleViews(titles []showbox.Title) []titleView {

	views := make([]titleView, 0, len(titles))

	for _, title := range titles {

		views = append(views, titleView{

			ID:      title.ID,
			BoxType: title.BoxType,

			Title: title.Title,
			Year:  title.Year,

			Poster:      proxy.ImageURL(title.Poster),
			Description: title.Description,
			Rating:      title.Rating,
		})

	}

	return views

}

func curatedViews(titles []tmdb.Title) []titleView {

	views := make([]titleView, 0, len(titles))

	for _, title := range titles {

		boxType := showbox.BoxTypeMovie

		if title.Kind == tmdb.KindTV {

			boxType = showbox.BoxTypeSeries

		}

		views = append(views, titleView{

			ID:      strconv.Itoa(title.ID),
			BoxType: boxType,
			Source:  "tmdb",
			TMDBID:  title.ID,

			Title: title.Title,
			Year:  title.Year,

			Poster:      proxy.ImageURL(title.Poster),
			Description: title.Description,
			Rating:      title.Rating,
			Genres:      title.Genres,
		})

	}

	return views

}

func prefer(primary string, fallback string) string {

	if primary != "" {

		return primary

	}

	return fallback

}

func normalizedTitle(value string) string {

	var normalized strings.Builder

	for _, char := range strings.ToLower(value) {

		if unicode.IsLetter(char) || unicode.IsDigit(char) {

			normalized.WriteRune(char)

		}

	}

	return normalized.String()

}

// Detail modules answer with a year that is sometimes quoted; the grid and the detail view want the same shape.
func year(raw string) int {

	value, err := strconv.Atoi(strings.TrimSpace(raw))

	if err != nil {

		return 0

	}

	return value

}

func key(season int, episode int) string {

	return strconv.Itoa(season) + "x" + strconv.Itoa(episode)

}
