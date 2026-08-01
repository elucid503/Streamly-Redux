package sports

import "time"

// Team is one side of a fixture.
type Team struct {

	Name string
	Logo string
	Abbreviation string

}

// MatchedChannel is an optional catalog channel suggested for watching.
type MatchedChannel struct {

	ID string
	Name string
	Logo string

}

// Match is a sports fixture from scoreboard sources — not a stream provider.
type Match struct {

	ID string
	Title string
	Category string
	League string

	StartTime time.Time
	Live bool

	HomeTeam *Team
	AwayTeam *Team

	HomeScore *int
	AwayScore *int
	StatusDetail string
	// Status is lifecycle state: pre / in / post.
	Status string

	// Broadcasts are live TV/stream outlets from ESPN for this event, preference-ordered.
	Broadcasts []string

	// Broadcast is the primary outlet label for display.
	Broadcast string

	Channel *MatchedChannel

}

const (
	StatusPre = "pre"
	StatusIn = "in"
	StatusPost = "post"
)
