package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"streamly/internal/proxy"
	"streamly/internal/sources/introdb"
	"streamly/internal/sources/showbox"
	"streamly/internal/sources/subdl"
	"streamly/internal/sources/tmdb"

	"github.com/gin-gonic/gin"
)

// Tight cap after Streamly-Web-style filtering — enough alternatives for a bad release match without flooding the menu.
const maxTracks = 8

type trackView struct {

	ID string `json:"id"`
	Label string `json:"label"`

	URL string `json:"url"`

}

// Candidates are ranked but nothing is loaded until someone asks — subtitles are shared, so they default to off (see _docs/DESIGN.md §5.5).
func (a *api) subtitles(c *gin.Context) {

	if !a.subdl.Configured() {

		c.JSON(http.StatusOK, gin.H{"tracks": []trackView{}})
		return

	}

	query := subdl.Query{

		ImdbID: c.Query("imdbId"),
		TmdbID: intQuery(c, "tmdbId"),

		Series: c.Query("series") == "1",

		Season: intQuery(c, "season"),
		Episode: intQuery(c, "episode"),

		ReleaseName: c.Query("release"),

	}

	// Streamly-Web resolves IMDB/TMDB from the media catalog; clients often omit them
	// (movies skip the detail page, history used to drop them). Fill from Showbox when needed.
	if strings.TrimSpace(query.ImdbID) == "" && query.TmdbID <= 0 {

		imdbID, tmdbID := a.captionIDs(c.Request.Context(), c.Query("id"), intQuery(c, "boxType"), c.Query("source"))

		query.ImdbID = imdbID
		query.TmdbID = tmdbID

	}

	if strings.TrimSpace(query.ImdbID) == "" && query.TmdbID <= 0 {

		c.JSON(http.StatusOK, gin.H{"tracks": []trackView{}})
		return

	}

	releases, err := a.subdl.Search(c.Request.Context(), query)

	if err != nil {

		slog.Error("subtitle search failed", "imdb", query.ImdbID, "tmdb", query.TmdbID, "err", err)

		c.JSON(http.StatusBadGateway, gin.H{"error": "subtitle search failed"})
		return

	}

	tracks := make([]trackView, 0, min(len(releases), maxTracks))
	langCount := map[string]int{}

	for _, release := range releases {

		if len(tracks) >= maxTracks {

			break

		}

		label := trackLabel(release, langCount)

		params := url.Values{}
		params.Set("path", release.Path)

		if query.Season > 0 {

			params.Set("season", strconv.Itoa(query.Season))

		}

		if query.Episode > 0 {

			params.Set("episode", strconv.Itoa(query.Episode))

		}

		tracks = append(tracks, trackView{

			ID: release.Path,
			Label: label,

			URL: proxy.PathPrefix + "/api/subtitle?" + params.Encode(),

		})

	}

	c.JSON(http.StatusOK, gin.H{"tracks": tracks})

}

func (a *api) subtitle(c *gin.Context) {

	path := c.Query("path")

	if path == "" {

		c.JSON(http.StatusBadRequest, gin.H{"error": "path is required"})
		return

	}

	vtt, err := a.subdl.Track(c.Request.Context(), path, intQuery(c, "season"), intQuery(c, "episode"))

	if err != nil {

		slog.Error("subtitle download failed", "path", path, "err", err)

		if errors.Is(err, subdl.ErrNotConfigured) {

			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "subtitles are not configured"})
			return

		}

		if errors.Is(err, subdl.ErrDownloadBudget) {

			c.JSON(http.StatusTooManyRequests, gin.H{"error": "SubDL's daily download limit is used up — searching still works, downloading resets tomorrow"})
			return

		}

		c.JSON(http.StatusBadGateway, gin.H{"error": "subtitle unavailable"})
		return

	}

	c.Header("Cache-Control", "public, max-age=3600")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "GET, OPTIONS")
	c.Header("Cross-Origin-Resource-Policy", "cross-origin")
	// text/plain is less likely to be mishandled by intermediate proxies than text/vtt.
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.Header("X-Content-Type-Options", "nosniff")

	c.Data(http.StatusOK, "text/plain; charset=utf-8", vtt)

}

func (a *api) intro(c *gin.Context) {

	ranges, err := a.introdb.Intro(c.Request.Context(), introQuery(c))

	if err != nil {

		slog.Debug("intro lookup failed", "err", err)

	}

	if ranges == nil {

		ranges = []introdb.Range{}

	}

	c.JSON(http.StatusOK, gin.H{"intro": ranges})

}

func introQuery(c *gin.Context) introdb.Query {

	durationMs, _ := strconv.ParseInt(c.Query("durationMs"), 10, 64)

	return introdb.Query{

		TmdbID: intQuery(c, "tmdbId"),
		ImdbID: c.Query("imdbId"),

		Season: intQuery(c, "season"),
		Episode: intQuery(c, "episode"),

		DurationMs: durationMs,

	}

}

func intQuery(c *gin.Context, name string) int {

	value, err := strconv.Atoi(c.Query(name))

	if err != nil {

		return 0

	}

	return value

}

// captionIDs looks up external IDs the way Streamly-Web's MovieCaptionQuery / EpisodeCaptionQuery do.
func (a *api) captionIDs(ctx context.Context, id string, boxType int, source string) (imdbID string, tmdbID int) {

	id = strings.TrimSpace(id)

	if id == "" {

		return "", 0

	}

	if boxType == 0 {

		boxType = showbox.BoxTypeMovie

	}

	if source == "tmdb" {

		if parsed, err := strconv.Atoi(id); err == nil && parsed > 0 {

			tmdbID = parsed

		}

	}

	if a.showbox == nil {

		return "", tmdbID

	}

	var detail *showbox.Detail
	var err error

	if source == "tmdb" && a.tmdb != nil {

		tmdbNum, parseErr := strconv.Atoi(id)

		if parseErr != nil {

			return "", tmdbID

		}

		kind := tmdb.KindMovie

		if boxType == showbox.BoxTypeSeries {

			kind = tmdb.KindTV

		}

		curated, curatedErr := a.tmdb.Details(ctx, kind, tmdbNum)

		if curatedErr != nil {

			return "", tmdbNum

		}

		detail, err = a.resolveCurated(ctx, curated, boxType)

		if err != nil {

			return "", tmdbNum

		}

		return detail.ImdbID, curated.ID

	}

	if boxType == showbox.BoxTypeSeries {

		detail, err = a.showbox.SeriesDetail(ctx, id)

	} else {

		detail, err = a.showbox.MovieDetail(ctx, id)

	}

	if err != nil || detail == nil {

		slog.Debug("caption id lookup failed", "id", id, "boxType", boxType, "source", source, "err", err)

		return "", tmdbID

	}

	if detail.ImdbID != "" {

		imdbID = detail.ImdbID

	}

	if detail.TmdbID > 0 {

		tmdbID = detail.TmdbID

	}

	return imdbID, tmdbID

}

func trackLabel(release subdl.Release, langCount map[string]int) string {

	lang := "English"

	if code := strings.ToLower(strings.TrimSpace(release.Language)); code != "" && code != "en" && code != "eng" && code != "english" {

		lang = languageName(release.Language)

		if lang == "" {

			lang = "Track"

		}

	}

	key := lang

	if release.HearingImpaired {

		key += "+sdh"

	}

	langCount[key]++
	count := langCount[key]

	label := lang

	if release.HearingImpaired {

		label += " (SDH)"

	}

	if count > 1 {

		label += fmt.Sprintf(" · Option %d", count)

	}

	return label

}

func languageName(value string) string {

	upper := strings.ToUpper(strings.TrimSpace(value))

	if upper == "" {

		return ""

	}

	switch upper {

	case "EN", "ENG", "ENGLISH":

		return "English"

	case "ES", "SPA", "SPANISH":

		return "Spanish"

	case "FR", "FRE", "FRA", "FRENCH":

		return "French"

	case "DE", "GER", "DEU", "GERMAN":

		return "German"

	case "PT", "POR", "PORTUGUESE":

		return "Portuguese"

	case "IT", "ITA", "ITALIAN":

		return "Italian"

	case "JA", "JPN", "JAPANESE":

		return "Japanese"

	case "KO", "KOR", "KOREAN":

		return "Korean"

	case "ZH", "CHI", "ZHO", "CHINESE":

		return "Chinese"

	case "RU", "RUS", "RUSSIAN":

		return "Russian"

	case "AR", "ARA", "ARABIC":

		return "Arabic"

	case "HI", "HIN", "HINDI":

		return "Hindi"

	case "NL", "DUT", "NLD", "DUTCH":

		return "Dutch"

	case "PL", "POL", "POLISH":

		return "Polish"

	case "TR", "TUR", "TURKISH":

		return "Turkish"

	}

	for code, name := range map[string]string{

		"ENGLISH": "English",
		"SPANISH": "Spanish",
		"FRENCH": "French",
		"GERMAN": "German",
		"PORTUGUESE": "Portuguese",
		"ITALIAN": "Italian",
		"JAPANESE": "Japanese",
		"KOREAN": "Korean",
		"CHINESE": "Chinese",
		"RUSSIAN": "Russian",
	} {

		if strings.Contains(upper, code) {

			return name

		}

	}

	return ""

}
