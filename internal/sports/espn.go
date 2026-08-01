package sports

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	espnScoreboardBase = "https://site.api.espn.com/apis/site/v2/sports"
	espnFetchTimeout = 12 * time.Second
)

// leagues is the set of ESPN sport/league paths used for the sports feed.
var leagues = []struct {
	Path string
	Category string
	Label string
}{

	{"baseball/mlb", "baseball", "MLB"},
	{"basketball/nba", "basketball", "NBA"},
	{"basketball/wnba", "basketball", "WNBA"},
	{"basketball/mens-college-basketball", "basketball", "NCAAB"},
	{"football/nfl", "american-football", "NFL"},
	{"football/college-football", "american-football", "NCAAF"},
	{"hockey/nhl", "hockey", "NHL"},
	{"soccer/usa.1", "football", "MLS"},
	{"soccer/eng.1", "football", "Premier League"},
	{"soccer/esp.1", "football", "La Liga"},
	{"soccer/uefa.champions", "football", "UCL"},
	{"racing/f1", "motor-sports", "Formula 1"},
	{"golf/pga", "golf", "PGA"},
	{"mma/ufc", "mma", "UFC"},

}

type espnScoreboard struct {

	Events []espnEvent `json:"events"`

}

type espnEvent struct {

	ID string `json:"id"`
	Name string `json:"name"`
	ShortName string `json:"shortName"`
	Date string `json:"date"`
	Competitions []espnCompetition `json:"competitions"`
	Status espnStatus `json:"status"`

}

type espnCompetition struct {

	Competitors []espnCompetitor `json:"competitors"`
	Status espnStatus `json:"status"`
	StartDate string `json:"startDate"`

	Broadcast string `json:"broadcast"`
	Broadcasts []espnBroadcast `json:"broadcasts"`
	GeoBroadcasts []espnGeoBroadcast `json:"geoBroadcasts"`

}

type espnBroadcast struct {

	Market string `json:"market"`
	Names []string `json:"names"`

}

type espnGeoBroadcast struct {

	Type struct {

		ShortName string `json:"shortName"`

	} `json:"type"`

	Market struct {

		Type string `json:"type"`

	} `json:"market"`

	Media struct {

		ShortName string `json:"shortName"`

	} `json:"media"`

	Lang string `json:"lang"`
	Region string `json:"region"`

}

type espnCompetitor struct {

	HomeAway string `json:"homeAway"`
	Score string `json:"score"`
	Team espnTeam `json:"team"`

}

type espnTeam struct {

	DisplayName string `json:"displayName"`
	ShortDisplayName string `json:"shortDisplayName"`
	Name string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	Logo string `json:"logo"`

}

type espnStatus struct {

	Type espnStatusType `json:"type"`

}

type espnStatusType struct {

	State string `json:"state"`
	Completed bool `json:"completed"`
	Detail string `json:"detail"`
	ShortDetail string `json:"shortDetail"`
	Description string `json:"description"`

}

func fetchAllLeagues(client *http.Client) ([]Match, error) {

	var (
		mu sync.Mutex
		wg sync.WaitGroup
		out []Match
		errs []error
	)

	for _, league := range leagues {

		wg.Add(1)

		go func(path, category, label string) {

			defer wg.Done()

			matches, err := fetchLeague(client, path, category, label)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {

				errs = append(errs, err)
				return

			}

			out = append(out, matches...)

		}(league.Path, league.Category, league.Label)

	}

	wg.Wait()

	if len(out) == 0 && len(errs) > 0 {

		return nil, fmt.Errorf("sports: espn fetch failed: %v", errs[0])

	}

	return out, nil

}

func fetchLeague(client *http.Client, path, category, label string) ([]Match, error) {

	url := espnScoreboardBase + "/" + path + "/scoreboard"

	req, err := http.NewRequest(http.MethodGet, url, nil)

	if err != nil {

		return nil, err

	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Streamly/1.0)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)

	if err != nil {

		return nil, err

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return nil, fmt.Errorf("%s: status %d", path, resp.StatusCode)

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if err != nil {

		return nil, err

	}

	var board espnScoreboard

	if err := json.Unmarshal(body, &board); err != nil {

		return nil, err

	}

	matches := make([]Match, 0, len(board.Events))

	for _, event := range board.Events {

		if m, ok := matchFromEvent(event, category, label); ok {

			matches = append(matches, m)

		}

	}

	return matches, nil

}

func matchFromEvent(event espnEvent, category, label string) (Match, bool) {

	start := parseESPNTime(event.Date)

	var home, away *Team
	var homeScore, awayScore *int

	status := event.Status
	statusDetail := firstNonEmpty(status.Type.ShortDetail, status.Type.Detail, status.Type.Description)
	state := strings.ToLower(strings.TrimSpace(status.Type.State))

	if len(event.Competitions) > 0 {

		comp := event.Competitions[0]

		if strings.TrimSpace(comp.Status.Type.State) != "" {

			status = comp.Status
			statusDetail = firstNonEmpty(status.Type.ShortDetail, status.Type.Detail, statusDetail)
			state = strings.ToLower(strings.TrimSpace(status.Type.State))

		}

		if t := parseESPNTime(comp.StartDate); !t.IsZero() {

			start = t

		}

		for _, c := range comp.Competitors {

			team := &Team{

				Name: firstNonEmpty(c.Team.DisplayName, c.Team.ShortDisplayName, c.Team.Name),
				Logo: c.Team.Logo,
				Abbreviation: c.Team.Abbreviation,

			}

			var score *int

			if s, err := strconv.Atoi(strings.TrimSpace(c.Score)); err == nil {

				score = &s

			}

			switch strings.ToLower(c.HomeAway) {

			case "home":

				home = team
				homeScore = score

			case "away":

				away = team
				awayScore = score

			}

		}

	}

	if start.IsZero() {

		return Match{}, false

	}

	// Drop completed games older than 6 hours to keep the feed fresh.
	if state == StatusPost && time.Since(start) > 6*time.Hour {

		return Match{}, false

	}

	// Drop far-future games beyond ~7 days.
	if start.After(time.Now().Add(7 * 24 * time.Hour)) {

		return Match{}, false

	}

	title := strings.TrimSpace(event.Name)

	if title == "" && home != nil && away != nil {

		title = away.Name + " vs " + home.Name

	}

	if title == "" {

		return Match{}, false

	}

	id := event.ID

	if id == "" {

		id = fmt.Sprintf("%s-%d", category, start.Unix())

	}

	live := state == StatusIn

	var broadcasts []broadcastCandidate

	if len(event.Competitions) > 0 {

		broadcasts = parseESPNBroadcasts(event.Competitions[0])

	}

	broadcastLabels := make([]string, 0, len(broadcasts))

	for _, b := range broadcasts {

		if b.Name != "" {

			broadcastLabels = append(broadcastLabels, b.Name)

		}

	}

	return Match{

		ID: "espn-" + id,
		Title: title,
		Category: category,
		League: label,

		StartTime: start,
		Live: live,

		HomeTeam: home,
		AwayTeam: away,

		HomeScore: homeScore,
		AwayScore: awayScore,
		Broadcasts: broadcastLabels,
		Broadcast: primaryBroadcastLabel(broadcasts),
		StatusDetail: statusDetail,
		Status: normalizeStatus(state, live),

	}, true

}

// parseESPNBroadcasts extracts ordered TV/stream outlets for catalog matching.
// Linear TV and regional markets rank above pure national streaming apps.
func parseESPNBroadcasts(comp espnCompetition) []broadcastCandidate {

	var out []broadcastCandidate
	seen := map[string]bool{}

	add := func(name, kind, market string, prefer int) {

		name = strings.TrimSpace(name)

		if name == "" {

			return

		}

		key := strings.ToLower(name)

		if seen[key] {

			return

		}

		seen[key] = true
		out = append(out, broadcastCandidate{

			Name: name,
			Kind: strings.ToLower(strings.TrimSpace(kind)),
			Market: strings.ToLower(strings.TrimSpace(market)),
			Prefer: prefer,

		})

	}

	for _, g := range comp.GeoBroadcasts {

		name := strings.TrimSpace(g.Media.ShortName)

		if name == "" {

			continue

		}

		kind := strings.ToLower(g.Type.ShortName)
		market := strings.ToLower(g.Market.Type)
		prefer := 50

		switch kind {

		case "tv":

			prefer += 40

		case "streaming":

			prefer += 10

		case "radio":

			prefer -= 80

		}

		switch market {

		case "home":

			prefer += 35

		case "away":

			prefer += 18

		case "national":

			prefer += 5

		}

		if skipBroadcasts[normalizeKey(name)] {

			prefer -= 30

		}

		add(name, kind, market, prefer)

	}

	for _, b := range comp.Broadcasts {

		market := strings.ToLower(b.Market)

		for _, name := range b.Names {

			prefer := 40

			if market == "national" {

				prefer += 5

			} else if market == "home" || market == "away" {

				prefer += 20

			}

			if skipBroadcasts[normalizeKey(name)] {

				prefer -= 30

			}

			add(name, "", market, prefer)

		}

	}

	if comp.Broadcast != "" {

		for _, part := range strings.Split(comp.Broadcast, ",") {

			add(strings.TrimSpace(part), "", "", 20)

		}

	}

	for i := 0; i < len(out); i++ {

		for j := i + 1; j < len(out); j++ {

			if out[j].Prefer > out[i].Prefer {

				out[i], out[j] = out[j], out[i]

			}

		}

	}

	return out

}

func normalizeStatus(state string, live bool) string {

	switch state {

	case StatusPre, StatusIn, StatusPost:

		return state

	}

	if live {

		return StatusIn

	}

	return StatusPre

}

func parseESPNTime(raw string) time.Time {

	raw = strings.TrimSpace(raw)

	if raw == "" {

		return time.Time{}

	}

	if t, err := time.Parse(time.RFC3339, raw); err == nil {

		return t

	}

	if t, err := time.Parse("2006-01-02T15:04Z", raw); err == nil {

		return t

	}

	return time.Time{}

}

func firstNonEmpty(values ...string) string {

	for _, v := range values {

		if s := strings.TrimSpace(v); s != "" {

			return s

		}

	}

	return ""

}
