package proxy

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Discord serves the activity under this prefix; requests from the iframe carry it (see _docs/DESIGN.md §2.1).
const PathPrefix = "/.proxy"

const (
	mediaRoute = "/proxy/media"
	imageRoute = "/proxy/image"

	imageAttempts    = 3
	imageConcurrency = 6
)

var forwardedHeaders = []string{

	"Content-Type",
	"Content-Length",
	"Content-Range",
	"Accept-Ranges",
}

// Everything a media request needs to reach an upstream that gates on headers and to report its own failure.
type Media struct {
	URL    string
	Source string

	Referer string

	// Room the playback belongs to, so an upstream failure can reach the hub that owns it (§5.2).
	Room string
}

type Handler struct {
	http       *http.Client
	imageHTTP  *http.Client
	imageSlots chan struct{}
	// pacedImageSlots serialises Wikimedia-class hosts (one in flight at a time).
	pacedImageSlots chan struct{}
	images     *imageCache
	fonts      *fontCache

	allowAnyOrigin bool

	onFailure func(room string, target string)

	mu          sync.Mutex
	lastFailure map[string]time.Time

	imageMu        sync.Mutex
	imageInflight  map[string]*imageFlight
	imageHostNext  map[string]time.Time
}

func New(allowAnyOrigin bool) *Handler {

	transport := &http.Transport{

		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 20 * time.Second,
	}

	httpClient := &http.Client{

		Transport: transport,
	}

	imageClient := &http.Client{

		Transport: transport,
		Timeout:   30 * time.Second,
	}

	return &Handler{

		http:       httpClient,
		imageHTTP:  imageClient,
		imageSlots: make(chan struct{}, imageConcurrency),
		pacedImageSlots: make(chan struct{}, pacedImageConcurrency),
		images:     newImageCache(),
		fonts:      newFontCache(),

		allowAnyOrigin: allowAnyOrigin,

		lastFailure: map[string]time.Time{},

		imageInflight: map[string]*imageFlight{},
		imageHostNext: map[string]time.Time{},
	}

}

func (h *Handler) OnFailure(report func(room string, target string)) {

	h.onFailure = report

}

func MediaURL(media Media) string {

	query := url.Values{}

	query.Set("u", encode(media.URL))
	query.Set("s", media.Source)

	if media.Referer != "" {

		query.Set("r", encode(media.Referer))

	}

	if media.Room != "" {

		query.Set("room", media.Room)

	}

	return PathPrefix + mediaRoute + "?" + query.Encode()

}

// Posters, logos, and avatars are blocked at their origin by the activity CSP, so they come through here too (§2.2).
func ImageURL(target string) string {

	if target == "" {

		return ""

	}

	return PathPrefix + imageRoute + "?u=" + encode(target)

}

func (h *Handler) Media(c *gin.Context) {

	target, err := decode(c.Query("u"))

	if err != nil || target == nil {

		c.String(http.StatusBadRequest, "bad target")
		return

	}

	source := c.Query("s")
	room := c.Query("room")

	referer, _ := decode(c.Query("r"))

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)

	if err != nil {

		c.String(http.StatusBadRequest, "bad target")
		return

	}

	applyHeaders(req, source, referer)

	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {

		req.Header.Set("Range", rangeHeader)

	}

	resp, err := h.http.Do(req)

	if err != nil {

		slog.Error("upstream unreachable", "source", source, "target", target.String(), "err", err)

		h.reportFailure(c, room, target.String())

		c.String(http.StatusBadGateway, "upstream unreachable")
		return

	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {

		slog.Error("upstream error", "source", source, "target", target.String(), "status", resp.StatusCode)

		h.reportFailure(c, room, target.String())

		c.String(http.StatusBadGateway, "upstream error")
		return

	}

	if isManifest(target, resp.Header.Get("Content-Type")) {

		h.serveManifest(c, resp, target, source, referer, room)
		return

	}

	for _, name := range forwardedHeaders {

		if value := resp.Header.Get(name); value != "" {

			c.Header(name, value)

		}

	}

	c.Status(resp.StatusCode)

	if _, err := io.Copy(c.Writer, resp.Body); err != nil {

		slog.Debug("client disconnected mid-stream", "source", source, "err", err)

	}

}

func (h *Handler) Image(c *gin.Context) {

	target, err := decode(c.Query("u"))

	if err != nil || target == nil {

		c.String(http.StatusBadRequest, "bad target")
		return

	}

	image, err := h.loadImage(c.Request.Context(), target)

	if err != nil {

		c.Header("Cache-Control", "no-store")

		var unavailable *imageUnavailableError

		if errors.As(err, &unavailable) {

			secs := int(unavailable.retryAfter.Seconds())

			if secs < 1 {

				secs = 3

			}

			c.Header("Retry-After", strconv.Itoa(secs))
			c.Status(http.StatusServiceUnavailable)
			return

		}

		if errors.Is(err, context.Canceled) {

			c.Status(http.StatusRequestTimeout)
			return

		}

		slog.Warn("image upstream failed", "host", target.Host, "err", err)
		c.Header("Retry-After", "2")
		c.Status(http.StatusBadGateway)
		return

	}

	c.Header("Content-Type", image.contentType)
	c.Header("Cache-Control", "public, max-age=86400")

	c.Data(http.StatusOK, image.contentType, image.body)

}

func (h *Handler) OriginCheck() gin.HandlerFunc {

	return func(c *gin.Context) {

		if h.allowAnyOrigin || isDiscordOrigin(c.GetHeader("Origin")) || isDiscordOrigin(c.GetHeader("Referer")) {

			c.Next()
			return

		}

		c.AbortWithStatus(http.StatusForbidden)

	}

}

func (h *Handler) serveManifest(c *gin.Context, resp *http.Response, target *url.URL, source string, referer *url.URL, room string) {

	body, err := io.ReadAll(resp.Body)

	if err != nil {

		slog.Error("manifest read failed", "source", source, "target", target.String(), "err", err)

		h.reportFailure(c, room, target.String())

		c.String(http.StatusBadGateway, "manifest unreadable")
		return

	}

	refererValue := ""

	if referer != nil {

		refererValue = referer.String()

	}

	rewritten := rewriteManifest(body, target, func(absolute string) string {

		return MediaURL(Media{

			URL:    absolute,
			Source: source,

			Referer: refererValue,
			Room:    room,
		})

	})

	c.Header("Cache-Control", "no-store")

	c.Data(http.StatusOK, "application/vnd.apple.mpegurl", rewritten)

}

func isDiscordOrigin(value string) bool {

	if value == "" {

		return false

	}

	parsed, err := url.Parse(value)

	if err != nil {

		return false

	}

	return strings.HasSuffix(parsed.Hostname(), ".discordsays.com")

}

func encode(value string) string {

	return base64.RawURLEncoding.EncodeToString([]byte(value))

}

func decode(value string) (*url.URL, error) {

	if value == "" {

		return nil, nil

	}

	raw, err := base64.RawURLEncoding.DecodeString(value)

	if err != nil {

		return nil, err

	}

	return url.Parse(string(raw))

}
