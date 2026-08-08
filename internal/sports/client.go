package sports

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"streamly/internal/catalog"
	"streamly/internal/sources/ntv"
)

const matchesTTL = 2 * time.Minute

// Client fetches sports fixtures from ESPN and soft-links them to catalog channels by name.
type Client struct {

	http *http.Client
	catalog *catalog.Catalog
	ntv *ntv.Client

	mu sync.RWMutex
	matches []Match
	matchesAt time.Time

}

// New builds a sports Client. catalog may be nil (channel links omitted).
// ntv is optional; when set, team/OTT feeds from NTV fill gaps left by linear-only matching.
func New(cat *catalog.Catalog, backup *ntv.Client) *Client {

	return &Client{

		http: &http.Client{Timeout: espnFetchTimeout},
		catalog: cat,
		ntv: backup,

	}

}

// Matches returns live and upcoming fixtures, cached briefly.
func (c *Client) Matches() ([]Match, error) {

	c.mu.RLock()

	if time.Since(c.matchesAt) < matchesTTL && c.matches != nil {

		out := append([]Match(nil), c.matches...)
		c.mu.RUnlock()

		return out, nil

	}

	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Since(c.matchesAt) < matchesTTL && c.matches != nil {

		return append([]Match(nil), c.matches...), nil

	}

	matches, err := fetchAllLeagues(c.http)

	if err != nil {

		if c.matches != nil {

			slog.Warn("sports refresh failed, serving stale", "err", err)
			return append([]Match(nil), c.matches...), nil

		}

		return nil, err

	}

	c.attachChannels(matches)
	sortMatches(matches)

	c.matches = matches
	c.matchesAt = time.Now()

	return append([]Match(nil), matches...), nil

}

func (c *Client) attachChannels(matches []Match) {

	if c.catalog == nil {

		return

	}

	channels := c.catalog.Channels()
	ntvListing := c.ntvListing()

	if len(channels) == 0 && len(ntvListing) == 0 {

		return

	}

	for i := range matches {

		matched := matchChannel(matches[i], channels, ntvListing)
		ensureNTVAux(c.catalog, ntvListing, matched)
		matches[i].Channel = matched

	}

}

func (c *Client) ntvListing() []ntv.Channel {

	if c.ntv == nil {

		return nil

	}

	listing, err := c.ntv.Channels(context.Background())

	if err != nil {

		slog.Warn("sports ntv listing unavailable", "err", err)
		return nil

	}

	return listing

}

func sortMatches(matches []Match) {

	rank := func(m Match) int {

		switch m.Status {

		case StatusIn:

			return 0

		case StatusPre:

			return 1

		case StatusPost:

			return 2

		}

		if m.Live {

			return 0

		}

		return 1

	}

	sort.SliceStable(matches, func(i, j int) bool {

		ri, rj := rank(matches[i]), rank(matches[j])

		if ri != rj {

			return ri < rj

		}

		return matches[i].StartTime.Before(matches[j].StartTime)

	})

}
