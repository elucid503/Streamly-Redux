package proxy

import (
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	fontCSSRoute = "/proxy/fonts/css"
	fontFileRoute = "/proxy/fonts/file"

	fontCSSURL = "https://fonts.googleapis.com/css2"
	// Modern UA so Google returns compact woff2 rather than legacy formats.
	fontBrowserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

	fontCSSTTL = 6 * time.Hour
	fontFileTTL = 7 * 24 * time.Hour
	fontMaxBytes = 2 << 20
)

// RE2 has no backreferences, so quote variants are matched separately.
var gstaticURLPattern = regexp.MustCompile(`url\((?:'(https://fonts\.gstatic\.com/[^']+)'|"(https://fonts\.gstatic\.com/[^"]+)"|(https://fonts\.gstatic\.com/[^)]+))\)`)

type fontCacheEntry struct {

	body []byte
	contentType string
	expires time.Time

}

// FontCSSURL builds the activity-safe stylesheet URL for a Google Fonts family query
// (e.g. "Inter:wght@400;500;600;700"). CSS is rewritten so every font file also hits the proxy.
func FontCSSURL(family string) string {

	query := url.Values{}

	query.Set("family", family)
	query.Set("display", "swap")

	return PathPrefix + fontCSSRoute + "?" + query.Encode()

}

func FontFileURL(target string) string {

	return PathPrefix + fontFileRoute + "?u=" + encode(target)

}

// FontCSS fetches Google Fonts CSS and rewrites gstatic urls through FontFile.
func (h *Handler) FontCSS(c *gin.Context) {

	family := strings.TrimSpace(c.Query("family"))

	if family == "" {

		c.String(http.StatusBadRequest, "family is required")
		return

	}

	display := strings.TrimSpace(c.Query("display"))

	if display == "" {

		display = "swap"

	}

	cacheKey := "css:" + family + "|" + display

	if body, contentType, ok := h.getFontCache(cacheKey); ok {

		c.Header("Content-Type", contentType)
		c.Header("Cache-Control", "public, max-age=21600")
		c.Data(http.StatusOK, contentType, body)
		return

	}

	upstream, err := url.Parse(fontCSSURL)

	if err != nil {

		c.Status(http.StatusInternalServerError)
		return

	}

	query := upstream.Query()
	query.Set("family", family)
	query.Set("display", display)
	upstream.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, upstream.String(), nil)

	if err != nil {

		c.Status(http.StatusInternalServerError)
		return

	}

	req.Header.Set("User-Agent", fontBrowserUA)
	req.Header.Set("Accept", "text/css,*/*;q=0.1")

	resp, err := h.http.Do(req)

	if err != nil {

		slog.Warn("font css upstream failed", "family", family, "err", err)
		c.Status(http.StatusBadGateway)
		return

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		slog.Warn("font css upstream status", "family", family, "status", resp.StatusCode)
		c.Status(http.StatusBadGateway)
		return

	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, fontMaxBytes))

	if err != nil {

		c.Status(http.StatusBadGateway)
		return

	}

	rewritten := gstaticURLPattern.ReplaceAllStringFunc(string(raw), func(match string) string {

		sub := gstaticURLPattern.FindStringSubmatch(match)

		if len(sub) < 4 {

			return match

		}

		// Groups: 1 = single-quoted, 2 = double-quoted, 3 = bare.
		target := firstNonEmpty(sub[1], sub[2], sub[3])

		if target == "" {

			return match

		}

		return "url(" + FontFileURL(target) + ")"

	})

	body := []byte(rewritten)
	contentType := "text/css; charset=utf-8"

	h.putFontCache(cacheKey, body, contentType, fontCSSTTL)

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=21600")
	c.Data(http.StatusOK, contentType, body)

}

// FontFile proxies a single fonts.gstatic.com asset.
func (h *Handler) FontFile(c *gin.Context) {

	target, err := decode(c.Query("u"))

	if err != nil || target == nil {

		c.String(http.StatusBadRequest, "bad target")
		return

	}

	if !isAllowedFontHost(target.Hostname()) {

		c.String(http.StatusBadRequest, "host not allowed")
		return

	}

	cacheKey := "file:" + target.String()

	if body, contentType, ok := h.getFontCache(cacheKey); ok {

		c.Header("Content-Type", contentType)
		c.Header("Cache-Control", "public, max-age=604800")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Data(http.StatusOK, contentType, body)
		return

	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)

	if err != nil {

		c.Status(http.StatusInternalServerError)
		return

	}

	req.Header.Set("User-Agent", fontBrowserUA)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://fonts.googleapis.com/")

	resp, err := h.http.Do(req)

	if err != nil {

		slog.Warn("font file upstream failed", "target", target.String(), "err", err)
		c.Status(http.StatusBadGateway)
		return

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		slog.Warn("font file upstream status", "target", target.String(), "status", resp.StatusCode)
		c.Status(http.StatusBadGateway)
		return

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, fontMaxBytes))

	if err != nil {

		c.Status(http.StatusBadGateway)
		return

	}

	contentType := resp.Header.Get("Content-Type")

	if contentType == "" {

		contentType = fontContentType(target.Path)

	}

	h.putFontCache(cacheKey, body, contentType, fontFileTTL)

	c.Header("Content-Type", contentType)
	c.Header("Cache-Control", "public, max-age=604800")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Data(http.StatusOK, contentType, body)

}

func firstNonEmpty(values ...string) string {

	for _, value := range values {

		if value != "" {

			return value

		}

	}

	return ""

}

func isAllowedFontHost(host string) bool {

	host = strings.ToLower(host)

	return host == "fonts.gstatic.com" || strings.HasSuffix(host, ".fonts.gstatic.com")

}

func fontContentType(path string) string {

	switch {

	case strings.HasSuffix(path, ".woff2"):

		return "font/woff2"

	case strings.HasSuffix(path, ".woff"):

		return "font/woff"

	case strings.HasSuffix(path, ".ttf"):

		return "font/ttf"

	case strings.HasSuffix(path, ".otf"):

		return "font/otf"

	default:

		return "application/octet-stream"

	}

}

// Shared font cache lives on Handler via the same mutex as images would — dedicated maps below.
type fontCache struct {

	mu sync.Mutex
	entries map[string]fontCacheEntry

}

func newFontCache() *fontCache {

	return &fontCache{entries: map[string]fontCacheEntry{}}

}

func (h *Handler) getFontCache(key string) ([]byte, string, bool) {

	if h.fonts == nil {

		return nil, "", false

	}

	h.fonts.mu.Lock()
	defer h.fonts.mu.Unlock()

	entry, ok := h.fonts.entries[key]

	if !ok || time.Now().After(entry.expires) {

		if ok {

			delete(h.fonts.entries, key)

		}

		return nil, "", false

	}

	return entry.body, entry.contentType, true

}

func (h *Handler) putFontCache(key string, body []byte, contentType string, ttl time.Duration) {

	if h.fonts == nil {

		return

	}

	h.fonts.mu.Lock()
	defer h.fonts.mu.Unlock()

	// Bound memory: drop everything if the map gets large; fonts are few.
	if len(h.fonts.entries) > 64 {

		h.fonts.entries = map[string]fontCacheEntry{}

	}

	h.fonts.entries[key] = fontCacheEntry{

		body: body,
		contentType: contentType,
		expires: time.Now().Add(ttl),

	}

}

