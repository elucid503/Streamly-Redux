package catalog

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"sync"
	"time"

	"streamly/internal/sources/daddylive"
	"streamly/internal/sources/ntv"
)

// Provider listings rot and the reference set gains channels; neither changes fast enough to want more than this.
const refreshInterval = 6 * time.Hour

type Catalog struct {

	live *daddylive.Client
	ntv *ntv.Client

	http *http.Client

	mu sync.RWMutex
	channels []Channel
	// Playable outside the Live TV grid (NTV team/OTT feeds sports injects).
	aux map[string]Channel

}

func New(live *daddylive.Client, backup *ntv.Client) *Catalog {

	return &Catalog{

		live: live,
		ntv: backup,

		http: referenceClient(),
		aux: map[string]Channel{},

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

	if channel, ok := c.aux[id]; ok {

		return channel, true

	}

	return Channel{}, false

}

// EnsureAux registers a channel for resolve without listing it in Live TV.
// Sports uses this for NTV-only team/OTT feeds that have no iptv-org identity.
func (c *Catalog) EnsureAux(channel Channel) {

	if channel.ID == "" || len(channel.Sources) == 0 {

		return

	}

	c.mu.Lock()

	if c.aux == nil {

		c.aux = map[string]Channel{}

	}

	c.aux[channel.ID] = channel

	c.mu.Unlock()

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

	built, unmatched, assumed := build(references, listing)

	ntvMatched := 0

	if c.ntv != nil {

		backup, err := c.ntv.Channels(ctx)

		if err != nil {

			// DaddyLive-only catalog is still useful; NTV is a backup tier.
			slog.Error("ntv catalog list failed", "err", err)

		} else {

			built, ntvMatched = attachNTV(built, references, backup)

		}

	}

	c.mu.Lock()

	c.channels = built

	c.mu.Unlock()

	slog.Info("catalog built", "channels", len(built), "reference", len(references), "listed", len(listing), "unmatched", len(unmatched), "assumedCountry", assumed, "ntvSources", ntvMatched)

	return nil

}

func build(references []reference, listing []daddylive.Channel) ([]Channel, []string, int) {

	lookup := newIndex(references)

	byID := map[string]Channel{}

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
			byID[found.ID] = existing
			continue

		}

		byID[found.ID] = Channel{

			ID: found.ID,
			Name: found.Name,

			Country: found.Country,
			Network: found.Network,

			Categories: found.Categories,

			Logo: found.Logo,

			Sources: []Source{source},

		}

	}

	return finalize(byID), unmatched, assumed

}

// attachNTV matches NTV listings onto the same iptv-org identities and appends them as backups.
func attachNTV(channels []Channel, references []reference, listing []ntv.Channel) ([]Channel, int) {

	lookup := newIndex(references)

	byID := map[string]Channel{}

	for _, channel := range channels {

		byID[channel.ID] = channel

	}

	matched := 0

	for _, entry := range listing {

		title := entry.Name

		if entry.Code != "" {

			title = entry.Name + " " + entry.Code

		}

		result, ok := lookup.match(title)

		if !ok {

			// Retry without country suffix when the country token made the match ambiguous.
			result, ok = lookup.match(entry.Name)

		}

		if !ok {

			continue

		}

		found := result.reference

		source := Source{Provider: ProviderNTV, Ref: entry.ID}

		if existing, seen := byID[found.ID]; seen {

			if hasProvider(existing.Sources, ProviderNTV) {

				continue

			}

			existing.Sources = append(existing.Sources, source)
			byID[found.ID] = existing
			matched++
			continue

		}

		// NTV-only channels that match the reference set still belong in the map.
		byID[found.ID] = Channel{

			ID: found.ID,
			Name: found.Name,

			Country: found.Country,
			Network: found.Network,

			Categories: found.Categories,

			Logo: found.Logo,

			Sources: []Source{source},

		}

		matched++

	}

	return finalize(byID), matched

}

func finalize(byID map[string]Channel) []Channel {

	channels := make([]Channel, 0, len(byID))

	for _, channel := range byID {

		sortSources(channel.Sources)
		channels = append(channels, channel)

	}

	sort.Slice(channels, func(a int, b int) bool {

		if channels[a].Name != channels[b].Name {

			return channels[a].Name < channels[b].Name

		}

		return channels[a].ID < channels[b].ID

	})

	return channels

}
