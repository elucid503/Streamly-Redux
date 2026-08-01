package catalog

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"streamly/internal/sources/daddylive"
)

// Provider listings rot and the reference set gains channels; neither changes fast enough to want more than this.
const refreshInterval = 6 * time.Hour

type Catalog struct {
	live *daddylive.Client

	http *http.Client

	mu       sync.RWMutex
	channels []Channel
}

func New(live *daddylive.Client) *Catalog {

	return &Catalog{

		live: live,

		http: referenceClient(),
	}

}

func (c *Catalog) Channels() []Channel {

	c.mu.RLock()

	defer c.mu.RUnlock()

	out := make([]Channel, len(c.channels))

	copy(out, c.channels)

	return out

}

func (c *Catalog) Channel(id string) (Channel, bool) {

	c.mu.RLock()

	defer c.mu.RUnlock()

	for _, channel := range c.channels {

		if channel.ID == id {

			return channel, true

		}

	}

	return Channel{}, false

}

// Keeps serving the previous catalog if a refresh fails, since a stale channel list beats an empty one.
func (c *Catalog) Watch(ctx context.Context) {

	ticker := time.NewTicker(refreshInterval)

	defer ticker.Stop()

	for {

		select {

		case <-ctx.Done():

			return

		case <-ticker.C:

			if err := c.Refresh(ctx); err != nil {

				slog.Error("catalog refresh failed", "err", err)

			}

		}

	}

}

func (c *Catalog) Refresh(ctx context.Context) error {

	references, err := fetchReference(ctx, c.http)

	if err != nil {

		return err

	}

	listing, err := c.live.Channels(ctx)

	if err != nil {

		return err

	}

	built, _, assumed := build(references, listing)

	c.mu.Lock()

	c.channels = built

	c.mu.Unlock()

	slog.Info("catalog built", "channels", len(built), "reference", len(references), "listed", len(listing), "assumedCountry", assumed)

	return nil

}

func build(references []reference, listing []daddylive.Channel) ([]Channel, []string, int) {

	lookup := newIndex(references)

	byID := map[string]*Channel{}

	unmatched := []string{}

	assumed := 0

	for _, entry := range listing {

		result, ok := lookup.match(entry.Name)

		if !ok {

			unmatched = append(unmatched, entry.Name)
			continue

		}

		if result.assumed {

			assumed++

		}

		found := result.reference

		source := Source{Provider: ProviderDaddyLive, Ref: entry.Ref}

		if existing, seen := byID[found.ID]; seen {

			// A second listing for the same channel is a genuine redundancy, so it becomes a fallback.
			existing.Sources = append(existing.Sources, source)
			continue

		}

		byID[found.ID] = &Channel{

			ID:   found.ID,
			Name: found.Name,

			Country: found.Country,
			Network: found.Network,

			Categories: found.Categories,

			Logo: found.Logo,

			Sources: []Source{source},
		}

	}

	channels := make([]Channel, 0, len(byID))

	for _, channel := range byID {

		channels = append(channels, *channel)

	}

	sort.Slice(channels, func(a int, b int) bool {

		if channels[a].Name != channels[b].Name {

			return channels[a].Name < channels[b].Name

		}

		return channels[a].ID < channels[b].ID

	})

	return channels, unmatched, assumed

}
