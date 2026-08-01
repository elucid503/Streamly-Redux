package proxy

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	imageCacheTTL = 24 * time.Hour
	// Logos rarely change; keep paced-host hits out of the hot path as long as practical.
	pacedImageCacheTTL = 7 * 24 * time.Hour
	// Short negative entries stop a stampede without stranding the UI for long.
	imageNegativeTTL = 20 * time.Second
	imageCacheMax = 2048
	imageMaxBytes = 2 << 20

	// Detached from the caller's cancel so a quick tab switch does not abandon a
	// paced Wikimedia download that the next mount will need from cache.
	imageFetchTimeout = 45 * time.Second

	// Wikimedia (and similar) rate-limit hard under concurrent logo grids.
	// One in-flight request and a cool-down between them is far safer than staggered starts.
	pacedImageConcurrency = 1
	pacedImageMinGap = 750 * time.Millisecond
	pacedImageBackoffMin = 15 * time.Second
	pacedImageBackoffMax = 3 * time.Minute

	// Wikimedia wants a descriptive UA with contact — a bare Chrome string is treated as a bot scrape.
	pacedImageUA = "Streamly/1.0 (Discord activity; image proxy; +https://github.com/streamly)"
)

// Hosts that rate-limit hard under channel-grid load (iptv-org logos via Wikimedia).
var pacedImageHosts = map[string]struct{}{

	"upload.wikimedia.org": {},
	"commons.wikimedia.org": {},
	"wikipedia.org": {},
	"en.wikipedia.org": {},
}

type cachedImage struct {

	body []byte
	contentType string
	expires time.Time
	// negative marks a remembered failure so clients stop re-fetching for a while.
	negative bool

}

type imageFlight struct {

	done chan struct{}

	image *cachedImage
	err error

}

// imageCache is an LRU of proxied image bodies keyed by absolute URL.
type imageCache struct {

	mu sync.Mutex
	// entries maps URL → list element holding cacheItem.
	entries map[string]*list.Element
	order *list.List

}

type cacheItem struct {

	key string
	image *cachedImage

}

func newImageCache() *imageCache {

	return &imageCache{

		entries: map[string]*list.Element{},
		order: list.New(),

	}

}

func (c *imageCache) get(key string) (*cachedImage, bool) {

	c.mu.Lock()

	defer c.mu.Unlock()

	el, ok := c.entries[key]

	if !ok {

		return nil, false

	}

	item := el.Value.(*cacheItem)

	if time.Now().After(item.image.expires) {

		c.removeElement(el)
		return nil, false

	}

	c.order.MoveToFront(el)

	return item.image, true

}

func (c *imageCache) put(key string, image *cachedImage) {

	c.mu.Lock()

	defer c.mu.Unlock()

	if el, ok := c.entries[key]; ok {

		item := el.Value.(*cacheItem)
		item.image = image
		c.order.MoveToFront(el)
		return

	}

	el := c.order.PushFront(&cacheItem{key: key, image: image})
	c.entries[key] = el

	for c.order.Len() > imageCacheMax {

		c.removeElement(c.order.Back())

	}

}

func (c *imageCache) removeElement(el *list.Element) {

	if el == nil {

		return

	}

	item := el.Value.(*cacheItem)
	delete(c.entries, item.key)
	c.order.Remove(el)

}

// loadImage serves from the in-process cache when possible and coalesces concurrent
// fetches for the same URL so a channel grid does not stampede one upstream object.
func (h *Handler) loadImage(ctx context.Context, target *url.URL) (*cachedImage, error) {

	key := target.String()

	if cached, ok := h.images.get(key); ok {

		if cached.negative {

			return nil, &imageUnavailableError{host: target.Host, retryAfter: imageNegativeTTL}

		}

		return cached, nil

	}

	h.imageMu.Lock()

	if flight, ok := h.imageInflight[key]; ok {

		h.imageMu.Unlock()

		select {

		case <-flight.done:

			return flight.image, flight.err

		case <-ctx.Done():

			return nil, ctx.Err()

		}

	}

	flight := &imageFlight{done: make(chan struct{})}
	h.imageInflight[key] = flight
	h.imageMu.Unlock()

	// Detach from the request context: the first viewer may unmount while scrolling
	// or switching tabs, but the bytes are still worth caching for the rest.
	fetchCtx, cancel := context.WithTimeout(context.Background(), imageFetchTimeout)
	image, err := h.fetchImageBytes(fetchCtx, target)
	cancel()

	if err == nil {

		h.images.put(key, image)

	} else if shouldNegativeCache(err) {

		h.images.put(key, &cachedImage{

			negative: true,
			expires: time.Now().Add(imageNegativeTTL),
			contentType: "application/octet-stream",

		})

		err = &imageUnavailableError{host: target.Host, retryAfter: imageNegativeTTL}

	}

	flight.image = image
	flight.err = err
	close(flight.done)

	h.imageMu.Lock()
	delete(h.imageInflight, key)
	h.imageMu.Unlock()

	return image, err

}

type imageUnavailableError struct {

	host string
	retryAfter time.Duration

}

func (e *imageUnavailableError) Error() string {

	return e.host + " temporarily unavailable"

}

func shouldNegativeCache(err error) bool {

	if err == nil || errors.Is(err, context.Canceled) {

		return false

	}

	// Our own fetch timeout is not a host ban — allow a retry soon.
	if errors.Is(err, context.DeadlineExceeded) {

		return false

	}

	msg := err.Error()

	return strings.Contains(msg, " 429") ||
		strings.Contains(msg, "returned 429") ||
		strings.Contains(msg, "Too Many Requests") ||
		strings.Contains(msg, " 403") ||
		strings.Contains(msg, "returned 403")

}

func (h *Handler) fetchImageBytes(ctx context.Context, target *url.URL) (*cachedImage, error) {

	paced := isPacedImageHost(target.Host)

	// Global concurrency for normal CDNs; paced hosts get their own single-flight slot.
	if paced {

		if err := h.acquirePacedImage(ctx, target.Host); err != nil {

			return nil, err

		}

		defer h.releasePacedImage(target.Host)

	} else {

		select {

		case h.imageSlots <- struct{}{}:

			defer func() { <-h.imageSlots }()

		case <-ctx.Done():

			return nil, ctx.Err()

		}

	}

	var lastErr error

	for attempt := 0; attempt < imageAttempts; attempt++ {

		if attempt > 0 {

			if paced {

				if err := h.waitPacedImageGap(ctx, target.Host); err != nil {

					return nil, err

				}

			}

		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)

		if err != nil {

			return nil, err

		}

		if paced {

			req.Header.Set("User-Agent", pacedImageUA)

		} else {

			req.Header.Set("User-Agent", browserUA)

		}

		req.Header.Set("Accept", "image/avif,image/webp,image/*,*/*;q=0.8")

		resp, err := h.imageHTTP.Do(req)

		if err != nil {

			lastErr = err

		} else if resp.StatusCode < http.StatusBadRequest {

			image, readErr := readCachedImage(resp, paced)
			resp.Body.Close()

			if readErr != nil {

				return nil, readErr

			}

			return image, nil

		} else {

			status := resp.StatusCode
			retryAfter := resp.Header.Get("Retry-After")
			resp.Body.Close()

			lastErr = fmt.Errorf("%s returned %d", target.Host, status)

			if status == http.StatusTooManyRequests || status == http.StatusForbidden {

				h.backoffImageHost(target.Host, retryAfter, attempt)

				slog.Warn("image host rate limited", "host", target.Host, "status", status, "attempt", attempt+1, "retryAfter", retryAfter)

			} else if status < http.StatusInternalServerError {

				return nil, lastErr

			}

		}

		if attempt == imageAttempts-1 {

			break

		}

		delay := time.Duration(attempt+1) * 250 * time.Millisecond

		if paced {

			delay = pacedRetryDelay(attempt)

		}

		select {

		case <-ctx.Done():

			return nil, ctx.Err()

		case <-time.After(delay):

		}

	}

	return nil, lastErr

}

func pacedRetryDelay(attempt int) time.Duration {

	// 2s, 4s, 8s — separate from host-level backoff so we don't thrash.
	delay := time.Duration(1<<uint(attempt+1)) * time.Second

	if delay > 15*time.Second {

		delay = 15 * time.Second

	}

	return delay

}

func readCachedImage(resp *http.Response, paced bool) (*cachedImage, error) {

	limited := io.LimitReader(resp.Body, imageMaxBytes+1)
	body, err := io.ReadAll(limited)

	if err != nil {

		return nil, err

	}

	if len(body) > imageMaxBytes {

		return nil, fmt.Errorf("image exceeds %d bytes", imageMaxBytes)

	}

	contentType := resp.Header.Get("Content-Type")

	if contentType == "" {

		contentType = "image/jpeg"

	}

	ttl := imageCacheTTL

	if paced {

		ttl = pacedImageCacheTTL

	}

	return &cachedImage{

		body: body,
		contentType: contentType,
		expires: time.Now().Add(ttl),

	}, nil

}

// acquirePacedImage takes the single paced-host token and enforces the cool-down.
func (h *Handler) acquirePacedImage(ctx context.Context, host string) error {

	select {

	case h.pacedImageSlots <- struct{}{}:

	case <-ctx.Done():

		return ctx.Err()

	}

	if err := h.waitPacedImageGap(ctx, host); err != nil {

		<-h.pacedImageSlots
		return err

	}

	return nil

}

func (h *Handler) releasePacedImage(host string) {

	// Next request may start only after the min gap from *this* completion.
	h.imageMu.Lock()

	next := time.Now().Add(pacedImageMinGap)

	if next.After(h.imageHostNext[host]) {

		h.imageHostNext[host] = next

	}

	h.imageMu.Unlock()

	<-h.pacedImageSlots

}

func (h *Handler) waitPacedImageGap(ctx context.Context, host string) error {

	for {

		h.imageMu.Lock()
		wait := time.Until(h.imageHostNext[host])
		h.imageMu.Unlock()

		if wait <= 0 {

			return nil

		}

		timer := time.NewTimer(wait)

		select {

		case <-ctx.Done():

			timer.Stop()
			return ctx.Err()

		case <-timer.C:

		}

	}

}

func (h *Handler) backoffImageHost(host string, retryAfter string, attempt int) {

	if !isPacedImageHost(host) {

		return

	}

	delay := pacedImageBackoffMin * time.Duration(1<<uint(attempt))

	if delay > pacedImageBackoffMax {

		delay = pacedImageBackoffMax

	}

	if parsed, ok := parseRetryAfter(retryAfter); ok && parsed > delay {

		delay = parsed

		if delay > pacedImageBackoffMax {

			delay = pacedImageBackoffMax

		}

	}

	next := time.Now().Add(delay)

	h.imageMu.Lock()

	if next.After(h.imageHostNext[host]) {

		h.imageHostNext[host] = next

	}

	h.imageMu.Unlock()

}

func parseRetryAfter(value string) (time.Duration, bool) {

	value = strings.TrimSpace(value)

	if value == "" {

		return 0, false

	}

	if secs, err := strconv.Atoi(value); err == nil && secs > 0 {

		return time.Duration(secs) * time.Second, true

	}

	if when, err := http.ParseTime(value); err == nil {

		wait := time.Until(when)

		if wait > 0 {

			return wait, true

		}

	}

	return 0, false

}

func isPacedImageHost(host string) bool {

	host = strings.ToLower(host)

	if _, ok := pacedImageHosts[host]; ok {

		return true

	}

	// Catch CDN mirrors like upload.wikimedia.org subdomains.
	return strings.HasSuffix(host, ".wikimedia.org") || strings.HasSuffix(host, ".wikipedia.org")

}
