package proxy

import (
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Discord serves the activity under this prefix; requests from the iframe carry it (see _docs/DESIGN.md §2.1).
const PathPrefix = "/.proxy"

const mediaRoute = "/proxy/media"

var forwardedHeaders = []string{

	"Content-Type",
	"Content-Length",
	"Content-Range",
	"Accept-Ranges",

}

type Handler struct {

	http *http.Client

	allowAnyOrigin bool

}

func New(allowAnyOrigin bool) *Handler {

	transport := &http.Transport{

		Proxy: http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 20 * time.Second,

	}

	httpClient := &http.Client{

		Transport: transport,

	}

	return &Handler{

		http: httpClient,

		allowAnyOrigin: allowAnyOrigin,

	}

}

func MediaURL(target string, source string, referer string) string {

	query := url.Values{}

	query.Set("u", encode(target))
	query.Set("s", source)

	if referer != "" {

		query.Set("r", encode(referer))

	}

	return PathPrefix + mediaRoute + "?" + query.Encode()

}

func (h *Handler) Media(c *gin.Context) {

	target, err := decode(c.Query("u"))

	if err != nil || target == nil {

		c.String(http.StatusBadRequest, "bad target")
		return

	}

	source := c.Query("s")
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
		c.String(http.StatusBadGateway, "upstream unreachable")
		return

	}

	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {

		slog.Error("upstream error", "source", source, "target", target.String(), "status", resp.StatusCode)
		c.String(http.StatusBadGateway, "upstream error")
		return

	}

	if isManifest(target, resp.Header.Get("Content-Type")) {

		h.serveManifest(c, resp, target, source, referer)
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

func (h *Handler) OriginCheck() gin.HandlerFunc {

	return func(c *gin.Context) {

		if h.allowAnyOrigin || isDiscordOrigin(c.GetHeader("Origin")) || isDiscordOrigin(c.GetHeader("Referer")) {

			c.Next()
			return

		}

		c.AbortWithStatus(http.StatusForbidden)

	}

}

func (h *Handler) serveManifest(c *gin.Context, resp *http.Response, target *url.URL, source string, referer *url.URL) {

	body, err := io.ReadAll(resp.Body)

	if err != nil {

		slog.Error("manifest read failed", "source", source, "target", target.String(), "err", err)
		c.String(http.StatusBadGateway, "manifest unreadable")
		return

	}

	refererValue := ""

	if referer != nil {

		refererValue = referer.String()

	}

	rewritten := rewriteManifest(body, target, func(absolute string) string {

		return MediaURL(absolute, source, refererValue)

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
