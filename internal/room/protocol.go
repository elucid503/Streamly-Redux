package room

import (
	"streamly/internal/resolve"
)

const (

	ActionPlay = "play"
	ActionPause = "pause"
	ActionSeek = "seek"
	ActionNext = "next"
	ActionPrev = "prev"
	ActionSetItem = "setItem"
	ActionSetSubtitle = "setSubtitle"

)

const (

	OpAdd = "add"
	OpRemove = "remove"
	OpMove = "move"

)

const (

	NoticeFailover = "failover"
	NoticeError = "error"
	NoticeAction = "action"

)

type Subtitle struct {

	ID string `json:"id"`
	Label string `json:"label"`

	URL string `json:"url"`

}

type Participant struct {

	UserID string `json:"userId"`
	Name string `json:"name"`

	Avatar string `json:"avatar,omitempty"`

}

type Actor struct {

	UserID string `json:"userId"`
	Name string `json:"name"`

	Action string `json:"action"`
	At int64 `json:"at"`

}

// Sent whole on every change; it is a few hundred bytes and diffing it would only add bugs (see _docs/DESIGN.md §8).
type State struct {

	Item *resolve.Item `json:"item"`
	Playback *resolve.Playback `json:"playback"`

	Playing bool `json:"playing"`

	AnchorMs int64 `json:"anchorMs"`
	AnchorAt int64 `json:"anchorAt"`

	Subtitle *Subtitle `json:"subtitle"`

	Queue []resolve.Item `json:"queue"`
	QueueIndex int `json:"queueIndex"`

	LastActor *Actor `json:"lastActor"`

}

type ClientFrame struct {

	Type string `json:"type"`

	InstanceID string `json:"instanceId"`
	AccessToken string `json:"accessToken"`
	GuildID string `json:"guildId,omitempty"`

	T0 int64 `json:"t0"`

	Action string `json:"action"`
	PositionMs int64 `json:"positionMs"`

	Item *resolve.Item `json:"item"`
	Track *Subtitle `json:"track"`

	Op string `json:"op"`

	Index int `json:"index"`
	To int `json:"to"`

}

type ServerFrame struct {

	Type string `json:"type"`

	State *State `json:"state,omitempty"`
	Participants []Participant `json:"participants,omitempty"`

	ServerTime int64 `json:"serverTime,omitempty"`

	T0 int64 `json:"t0,omitempty"`
	T1 int64 `json:"t1,omitempty"`

	Kind string `json:"kind,omitempty"`
	Text string `json:"text,omitempty"`

}
