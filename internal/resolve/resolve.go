package resolve

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"streamly/internal/catalog"
	"streamly/internal/proxy"
	"streamly/internal/sources/daddylive"
	"streamly/internal/sources/febbox"
	"streamly/internal/sources/ntv"
	"streamly/internal/sources/showbox"
)

var (

	ErrNoSuchChannel = errors.New("resolve: channel is not in the map")
	ErrNoSourceLeft = errors.New("resolve: every source for this channel failed")
	ErrNoPlayableFile = errors.New("resolve: title has no playable file")

	// Kept distinct so an expired Febbox cookie surfaces as authentication failure rather than as "no qualities" (§5.4).
	ErrProviderAuth = errors.New("resolve: provider rejected our credentials")

)

type Resolver struct {

	catalog *catalog.Catalog

	live *daddylive.Client
	ntv *ntv.Client

	showbox *showbox.Client
	febbox *febbox.Client

}

func New(channels *catalog.Catalog, live *daddylive.Client, backup *ntv.Client, box *showbox.Client, files *febbox.Client) *Resolver {

	return &Resolver{

		catalog: channels,

		live: live,
		ntv: backup,

		showbox: box,
		febbox: files,

	}

}

// room tags the proxy URLs so an upstream failure can be attributed back to the room watching it (§5.2).
func (r *Resolver) Play(ctx context.Context, item Item, sourceIndex int, room string) (*Playback, error) {

	if item.Kind == KindChannel {

		return r.playChannel(ctx, item, sourceIndex, room)

	}

	return r.playVOD(ctx, item, room)

}

func (r *Resolver) playChannel(ctx context.Context, item Item, sourceIndex int, room string) (*Playback, error) {

	channel, ok := r.catalog.Channel(item.ID)

	if !ok {

		return nil, ErrNoSuchChannel

	}

	if sourceIndex >= len(channel.Sources) {

		return nil, ErrNoSourceLeft

	}

	for index := sourceIndex; index < len(channel.Sources); index++ {

		source := channel.Sources[index]

		stream, err := r.stream(ctx, source)

		if err != nil {

			slog.Error("channel source failed", "channel", channel.ID, "provider", source.Provider, "ref", source.Ref, "err", err)
			continue

		}

		slog.Info("channel resolved", "channel", channel.ID, "provider", source.Provider, "source", index)

		return &Playback{

			Kind: "hls",
			URL: proxy.MediaURL(proxy.Media{

				URL: stream.url,
				Source: source.Provider,

				Referer: stream.referer,
				Room: room,

			}),

			Provider: source.Provider,

			SourceIndex: index,
			SourceCount: len(channel.Sources),

		}, nil

	}

	return nil, ErrNoSourceLeft

}

type liveStream struct {

	url string
	referer string

}

func (r *Resolver) stream(ctx context.Context, source catalog.Source) (*liveStream, error) {

	switch source.Provider {

	case catalog.ProviderDaddyLive:

		resolved, err := r.live.Resolve(ctx, source.Ref)

		if err != nil {

			return nil, err

		}

		return &liveStream{url: resolved.URL, referer: resolved.Referer}, nil

	case catalog.ProviderNTV:

		resolved, err := r.ntv.Resolve(ctx, source.Ref)

		if err != nil {

			return nil, err

		}

		return &liveStream{url: resolved.URL, referer: resolved.Referer}, nil

	}

	return nil, fmt.Errorf("resolve: unknown provider %q", source.Provider)

}

func (r *Resolver) playVOD(ctx context.Context, item Item, room string) (*Playback, error) {

	boxType := item.BoxType

	if boxType == 0 {

		boxType = showbox.BoxTypeMovie

	}

	shareKey, err := r.showbox.ShareKey(ctx, item.ID, boxType)

	if err != nil {

		return nil, err

	}

	file, err := r.file(ctx, shareKey, item, boxType)

	if err != nil {

		return nil, err

	}

	qualities, err := r.febbox.Qualities(ctx, file.FileID)

	if errors.Is(err, febbox.ErrNotAuthenticated) {

		return nil, ErrProviderAuth

	}

	if err != nil {

		return nil, err

	}

	renditions := make([]Quality, 0, len(qualities))

	for _, quality := range qualities {

		label := quality.Quality

		if label == "" {

			label = quality.Name

		}

		// ORG/source copies are raw archives, not useful playback choices.
		if isOriginalLabel(label) {

			continue

		}

		renditions = append(renditions, Quality{

			Label: label,
			URL: proxy.MediaURL(proxy.Media{

				URL: quality.URL,
				Source: "febbox",

				Room: room,

			}),

			Height: heightOf(label),

		})

	}

	sortByHeight(renditions)

	chosen := defaultRendition(renditions)

	if chosen == nil {

		return nil, ErrNoPlayableFile

	}

	slog.Info("vod resolved", "title", item.Title, "file", file.Name, "renditions", len(renditions), "default", chosen.Label)

	return &Playback{

		Kind: "file",
		URL: chosen.URL,

		Qualities: renditions,

		Provider: "febbox",
		ReleaseName: file.Name,

		SourceCount: 1,

	}, nil

}

func (r *Resolver) file(ctx context.Context, shareKey string, item Item, boxType int) (*febbox.File, error) {

	files, err := r.febbox.Files(ctx, shareKey, "0")

	if err != nil {

		return nil, err

	}

	if boxType == showbox.BoxTypeMovie {

		return pickVideo(files)

	}

	for _, entry := range files {

		if !entry.IsDir || !matchesSeason(entry.Name, item.Season) {

			continue

		}

		episodes, err := r.febbox.Files(ctx, shareKey, entry.FileID)

		if err != nil {

			return nil, err

		}

		if file := pickEpisode(episodes, item.Season, item.Episode); file != nil {

			return file, nil

		}

	}

	// Some series are flat rather than foldered per season.
	if file := pickEpisode(files, item.Season, item.Episode); file != nil {

		return file, nil

	}

	return nil, ErrNoPlayableFile

}
