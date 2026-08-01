package resolve

import (
	"fmt"
)

type Kind string

const (

	KindChannel Kind = "channel"
	KindVOD Kind = "vod"

)

// What the room is watching. Everything needed to re-resolve it later, and nothing about how it is being played.
type Item struct {

	Kind Kind `json:"kind"`

	ID string `json:"id"`
	Title string `json:"title"`

	Poster string `json:"poster,omitempty"`
	Caption string `json:"caption,omitempty"`

	BoxType int `json:"boxType,omitempty"`

	Season int `json:"season,omitempty"`
	Episode int `json:"episode,omitempty"`

	EpisodeTitle string `json:"episodeTitle,omitempty"`
	Description string `json:"description,omitempty"`

	ImdbID string `json:"imdbId,omitempty"`
	TmdbID int `json:"tmdbId,omitempty"`

}

func (i Item) Key() string {

	if i.Kind == KindChannel {

		return string(i.Kind) + ":" + i.ID

	}

	return fmt.Sprintf("%s:%s:%d:%d", i.Kind, i.ID, i.Season, i.Episode)

}

type Quality struct {

	Label string `json:"label"`
	URL string `json:"url"`

	Size string `json:"size,omitempty"`
	Height int `json:"height"`

}

// How to play the item right now. Rebuilt on every source change, never persisted.
type Playback struct {

	Kind string `json:"kind"`
	URL string `json:"url"`

	Qualities []Quality `json:"qualities,omitempty"`

	Provider string `json:"provider"`
	ReleaseName string `json:"releaseName,omitempty"`

	SourceIndex int `json:"sourceIndex"`
	SourceCount int `json:"sourceCount"`

}
