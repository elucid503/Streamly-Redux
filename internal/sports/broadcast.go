package sports

import (
	"strings"

	"streamly/internal/catalog"
)

// broadcastCandidate is a live TV/stream outlet from ESPN for one event.
type broadcastCandidate struct {

	Name string
	// Kind is "tv", "streaming", "radio", or "".
	Kind string
	// Market is "national", "home", "away", or "".
	Market string
	// Prefer is a ranking boost (higher = try first).
	Prefer int

}

// skipBroadcasts are OTT-only outlets we do not map to linear catalog channels.
var skipBroadcasts = map[string]bool{

	"peacock": true,
	"netflix": true,
	"apple tv": true,
	"apple tv+": true,
	"amazon": true,
	"amazon prime": true,
	"prime video": true,
	"max": true,
	"hulu": true,
	"youtube": true,
	"youtube tv": true,
	"paramount+": true,
	"paramount plus": true,
	"disney+": true,
	"fubo": true,
	"mlb.tv": true,
	"nba league pass": true,
	"nhl.tv": true,
	"espn+": true,
	"espn unlmtd": true,
	"espn unlimited": true,
	"radio": true,
	"siriusxm": true,
	"eradm": true,

}

// broadcastAliases maps ESPN short names → catalog search labels (ordered).
var broadcastAliases = map[string][]string{

	"sny": {"SportsNet New York", "SNY"},
	"sportsnet new york": {"SportsNet New York", "SNY"},
	"yes": {"YES Network", "YES"},
	"yes network": {"YES Network", "YES"},
	"nesn": {"NESN"},
	"nesn plus": {"NESN Plus", "NESN"},
	"masn": {"MASN"},
	"masn2": {"MASN"},
	"marquee sports net": {"Marquee Sports Network", "Marquee"},
	"marquee": {"Marquee Sports Network"},
	"sportsnet la": {"Spectrum SportsNet LA", "SportsNet LA"},
	"spectrum sportsnet la": {"Spectrum SportsNet LA"},
	"spectrum sportsnet": {"Spectrum SportsNet LA"},
	"spectrum sports net": {"Spectrum SportsNet LA"},
	"nbc sports ba": {"NBC Sports Bay Area"},
	"nbc sports bay area": {"NBC Sports Bay Area"},
	"nbc sports ca": {"NBC Sports California"},
	"nbc sports california": {"NBC Sports California"},
	"sportsnet": {"Sportsnet One", "Sportsnet 360"},
	"nbc sports boston": {"NBC Sports Boston"},
	"nbc sports chicago": {"NBC Sports Chicago"},
	"nbc sports now": {"NBC Sports NOW"},
	"space city home network": {"Space City Home Network", "FanDuel Sports Network Southwest Houston"},
	"chsn": {"Chicago Sports Network", "NBC Sports Chicago"},
	"chicago sports network": {"Chicago Sports Network", "NBC Sports Chicago"},
	"rangers sports network": {"FanDuel Sports Network Southwest", "Rangers Sports Network"},
	"sportsnet/tva": {"Sportsnet One", "Sportsnet 360", "TVA Sports"},
	"sportsnet one": {"Sportsnet One"},
	"sportsnet 360": {"Sportsnet 360"},
	"tva": {"TVA Sports"},
	"tva sports": {"TVA Sports"},
	"msg": {"MSG"},
	"msg 2": {"MSG 2", "MSG"},
	"msg plus": {"MSG Plus"},
	"msg western new york": {"MSG Western New York", "MSG"},
	"altitude": {"Altitude Sports"},
	"altitude sports": {"Altitude Sports"},
	"root sports northwest": {"Root Sports Northwest"},
	"root sports": {"Root Sports Northwest"},
	"bravesvision": {"FanDuel Sports Network Southeast", "BravesVision"},
	"bally sports": {"FanDuel Sports Network"},

	"espn": {"ESPN"},
	"espn2": {"ESPN2", "ESPN 2"},
	"espnu": {"ESPNU"},
	"espn news": {"ESPNews"},
	"abc": {"ABC"},
	"nbc": {"NBC"},
	"cbs": {"CBS"},
	"fox": {"Fox", "FOX"},
	"fs1": {"Fox Sports 1"},
	"fs2": {"Fox Sports 2"},
	"fox sports 1": {"Fox Sports 1"},
	"fox sports 2": {"Fox Sports 2"},
	"tnt": {"TNT"},
	"tbs": {"TBS"},
	"usa": {"USA Network"},
	"usa network": {"USA Network"},
	"nfl network": {"NFL Network"},
	"nfl redzone": {"NFL RedZone", "NFL Network"},
	"mlb network": {"MLB Network"},
	"nba tv": {"NBA TV"},
	"nhl network": {"NHL Network"},
	"golf channel": {"Golf Channel"},
	"cbs sports network": {"CBS Sports Network"},
	"bein sports": {"beIN Sports"},
	"nbc/peacock": {"NBC", "Peacock"},
	"espn/abc": {"ESPN", "ABC"},
	"espn unlmtd/mlb.tv": {"ESPN", "MLB Network"},

}

// teamRSNs maps team name fragments → preferred regional sports networks.
var teamRSNs = map[string][]string{

	"yankees": {"YES Network", "YES"},
	"mets": {"SportsNet New York", "SNY"},
	"red sox": {"NESN"},
	"dodgers": {"Spectrum SportsNet LA", "SportsNet LA"},
	"cubs": {"Marquee Sports Network"},
	"white sox": {"Chicago Sports Network", "NBC Sports Chicago", "CHSN"},
	"giants": {"NBC Sports Bay Area"},
	"athletics": {"NBC Sports California"},
	"angels": {"FanDuel Sports Network West"},
	"padres": {"FanDuel Sports Network", "Padres.TV"},
	"mariners": {"Root Sports Northwest"},
	"astros": {"Space City Home Network", "FanDuel Sports Network Southwest Houston"},
	"phillies": {"NBC Sports Philadelphia", "NBC Sports"},
	"braves": {"FanDuel Sports Network Southeast"},
	"orioles": {"MASN"},
	"nationals": {"MASN"},
	"blue jays": {"Sportsnet", "Sportsnet One"},
	"guardians": {"FanDuel Sports Network Great Lakes", "FanDuel Sports Network Ohio Cleveland"},
	"tigers": {"FanDuel Sports Network Detroit"},
	"royals": {"FanDuel Sports Network Kansas City"},
	"twins": {"FanDuel Sports Network North", "Twins.TV"},
	"brewers": {"FanDuel Sports Network Wisconsin"},
	"cardinals": {"FanDuel Sports Network Midwest"},
	"reds": {"FanDuel Sports Network Ohio Cincinnati", "FanDuel Sports Network Ohio"},
	"pirates": {"SportsNet Pittsburgh", "AT&T SportsNet Pittsburgh"},
	"rockies": {"Altitude Sports"},
	"diamondbacks": {"FanDuel Sports Network Arizona"},
	"marlins": {"FanDuel Sports Network Florida"},
	"rays": {"FanDuel Sports Network Sun"},

	"knicks": {"MSG"},
	"nets": {"YES Network", "YES"},
	"lakers": {"Spectrum SportsNet", "Spectrum SportsNet LA"},
	"clippers": {"FanDuel Sports Network SoCal", "FanDuel Sports Network West"},
	"celtics": {"NBC Sports Boston", "NESN"},
	"warriors": {"NBC Sports Bay Area"},
	"bulls": {"Chicago Sports Network", "NBC Sports Chicago"},
	"heat": {"FanDuel Sports Network Sun", "FanDuel Sports Network Florida"},
	"bucks": {"FanDuel Sports Network Wisconsin"},
	"mavericks": {"FanDuel Sports Network Southwest"},
	"rockets": {"Space City Home Network", "FanDuel Sports Network Southwest Houston"},
	"spurs": {"FanDuel Sports Network Southwest San Antonio"},
	"suns": {"FanDuel Sports Network Arizona"},
	"nuggets": {"Altitude Sports"},
	"timberwolves": {"FanDuel Sports Network North"},
	"thunder": {"FanDuel Sports Network Oklahoma"},
	"jazz": {"Jazz+", "KJZZ"},
	"kings": {"NBC Sports California"},
	"blazers": {"Root Sports Northwest"},
	"trail blazers": {"Root Sports Northwest"},
	"pelicans": {"FanDuel Sports Network New Orleans", "Gulf Coast Sports"},
	"grizzlies": {"FanDuel Sports Network Southeast"},
	"hornets": {"FanDuel Sports Network Southeast"},
	"hawks": {"FanDuel Sports Network Southeast"},
	"magic": {"FanDuel Sports Network Florida"},
	"wizards": {"Monumental Sports Network"},
	"pistons": {"FanDuel Sports Network Detroit"},
	"pacers": {"FanDuel Sports Network Indiana"},
	"cavaliers": {"FanDuel Sports Network Ohio Cleveland"},
	"cavs": {"FanDuel Sports Network Ohio Cleveland"},
	"raptors": {"Sportsnet", "TSN"},
	"76ers": {"NBC Sports Philadelphia"},
	"sixers": {"NBC Sports Philadelphia"},

	"islanders": {"MSG", "MSG Plus"},
	"devils": {"MSG", "MSG Sportsnet"},
	"bruins": {"NESN"},
	"maple leafs": {"Sportsnet", "TSN"},
	"canadiens": {"TSN", "RDS"},
	"penguins": {"SportsNet Pittsburgh"},
	"capitals": {"Monumental Sports Network"},
	"flyers": {"NBC Sports Philadelphia"},
	"blackhawks": {"Chicago Sports Network", "NBC Sports Chicago"},
	"red wings": {"FanDuel Sports Network Detroit"},
	"blues": {"FanDuel Sports Network Midwest"},
	"avalanche": {"Altitude Sports"},
	"stars": {"FanDuel Sports Network Southwest"},
	"predators": {"FanDuel Sports Network South"},
	"hurricanes": {"FanDuel Sports Network South"},
	"panthers": {"FanDuel Sports Network Florida"},
	"lightning": {"FanDuel Sports Network Sun"},
	"wild": {"FanDuel Sports Network North"},
	"jets": {"TSN"},
	"oilers": {"Sportsnet"},
	"flames": {"Sportsnet"},
	"canucks": {"Sportsnet"},
	"kraken": {"Root Sports Northwest"},
	"sharks": {"NBC Sports California"},
	"ducks": {"Victory+", "FanDuel Sports Network SoCal"},
	"golden knights": {"SCRIPPS", "KMCC"},

}

// nfl/mlb disambiguation for "rangers" / "kings"
var teamRSNsByCategory = map[string]map[string][]string{

	"baseball": {
		"rangers": {"FanDuel Sports Network Southwest", "Rangers Sports Network"},
	},
	"hockey": {
		"rangers": {"MSG"},
		"kings": {"FanDuel Sports Network SoCal", "FanDuel Sports Network West"},
	},
	"basketball": {
		"kings": {"NBC Sports California"},
		"rangers": {},
	},

}

// categoryDefaultNetworks is the last-resort national lineup.
var categoryDefaultNetworks = map[string][]string{

	"baseball": {"MLB Network", "ESPN", "TBS", "Fox", "FOX"},
	"basketball": {"NBA TV", "ESPN", "TNT", "ABC"},
	"american-football": {"NFL Network", "ESPN", "Fox", "FOX", "CBS", "NBC"},
	"hockey": {"NHL Network", "ESPN", "TNT"},
	"football": {"ESPN", "Fox Sports 1", "FS1", "beIN Sports"},
	"motor-sports": {"ESPN", "ABC", "Fox Sports 1"},
	"golf": {"Golf Channel", "CBS", "NBC"},
	"mma": {"ESPN", "UFC Fight Pass"},

}

// matchChannel picks the best catalog channel for a fixture using live ESPN
// broadcast data first, then team RSN maps, then national defaults.
func matchChannel(m Match, channels []catalog.Channel) *MatchedChannel {

	if len(channels) == 0 {

		return nil

	}

	for _, term := range expandBroadcastLabels(m.Broadcasts) {

		if ch := findCatalogChannel(channels, term); ch != nil {

			return ch

		}

	}

	for _, term := range teamSearchTerms(m) {

		if ch := findCatalogChannel(channels, term); ch != nil {

			return ch

		}

	}

	for _, name := range categoryDefaultNetworks[m.Category] {

		if ch := findCatalogChannel(channels, name); ch != nil {

			return ch

		}

	}

	return nil

}

func expandBroadcastLabels(names []string) []string {

	var terms []string
	seen := map[string]bool{}

	add := func(label string) {

		label = strings.TrimSpace(label)

		if label == "" {

			return

		}

		key := normalizeKey(label)

		if seen[key] || skipBroadcasts[key] {

			return

		}

		seen[key] = true
		terms = append(terms, label)

	}

	for _, name := range names {

		key := normalizeKey(name)

		if key == "" {

			continue

		}

		parts := splitBroadcastName(name)

		if len(parts) == 0 {

			parts = []string{name}

		}

		for _, part := range parts {

			pk := normalizeKey(part)

			if pk == "" || skipBroadcasts[pk] {

				continue

			}

			if aliases, ok := broadcastAliases[pk]; ok {

				for _, a := range aliases {

					add(a)

				}

				continue

			}

			add(part)

		}

		if aliases, ok := broadcastAliases[key]; ok {

			for _, a := range aliases {

				add(a)

			}

		}

	}

	return terms

}

func teamSearchTerms(m Match) []string {

	var terms []string
	seen := map[string]bool{}

	addAll := func(labels []string) {

		for _, l := range labels {

			k := normalizeKey(l)

			if l == "" || seen[k] {

				continue

			}

			seen[k] = true
			terms = append(terms, l)

		}

	}

	if m.HomeTeam != nil {

		addAll(rsnForTeam(m.Category, m.HomeTeam.Name))

	}

	if m.AwayTeam != nil {

		addAll(rsnForTeam(m.Category, m.AwayTeam.Name))

	}

	return terms

}

func rsnForTeam(category, teamName string) []string {

	n := normalizeKey(teamName)

	if n == "" {

		return nil

	}

	if byCat := teamRSNsByCategory[category]; byCat != nil {

		for key, labels := range byCat {

			if strings.Contains(n, key) && len(labels) > 0 {

				return labels

			}

		}

	}

	var bestKey string
	var best []string

	for key, labels := range teamRSNs {

		if len(labels) == 0 {

			continue

		}

		if strings.Contains(n, key) && len(key) > len(bestKey) {

			bestKey = key
			best = labels

		}

	}

	return best

}

func findCatalogChannel(channels []catalog.Channel, name string) *MatchedChannel {

	name = strings.TrimSpace(name)

	if name == "" {

		return nil

	}

	norm := normalizeKey(name)
	var best *catalog.Channel
	bestScore := 0

	for i := range channels {

		ch := &channels[i]
		cn := normalizeKey(ch.Name)
		score := 0

		if cn == norm {

			score = 100

		} else if strings.HasPrefix(cn, norm) && len(norm) >= 4 {

			score = 85

		} else if strings.HasPrefix(norm, cn) && len(cn) >= 5 {

			score = 75

		} else if strings.Contains(cn, norm) && len(norm) >= 5 {

			qWords := len(strings.Fields(norm))
			nWords := len(strings.Fields(cn))

			if nWords > qWords+1 {

				score = 40

			} else {

				score = 65

			}

		} else if strings.Contains(norm, cn) && len(cn) >= 6 {

			score = 55

		}

		if score > 0 && hasCategory(ch.Categories, "sports") {

			score += 8

		}

		if score > 0 && strings.EqualFold(ch.Country, "US") {

			score += 3

		}

		if score > bestScore {

			bestScore = score
			best = ch

		}

	}

	// High bar avoids "Rangers" → random AFN Sports style false positives.
	if best == nil || bestScore < 70 {

		return nil

	}

	return &MatchedChannel{ID: best.ID, Name: best.Name, Logo: best.Logo}

}

func hasCategory(categories []string, want string) bool {

	for _, c := range categories {

		if strings.EqualFold(c, want) {

			return true

		}

	}

	return false

}

func splitBroadcastName(name string) []string {

	name = strings.TrimSpace(name)

	if name == "" {

		return nil

	}

	for _, sep := range []string{"/", "|"} {

		if !strings.Contains(name, sep) {

			continue

		}

		var out []string

		for _, piece := range strings.Split(name, sep) {

			piece = strings.TrimSpace(piece)

			if piece != "" {

				out = append(out, piece)

			}

		}

		if len(out) > 0 {

			return out

		}

	}

	return []string{name}

}

func normalizeKey(s string) string {

	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "’", "")
	s = strings.Join(strings.Fields(s), " ")

	return s

}

// primaryBroadcastLabel picks a human-facing network label for the UI.
func primaryBroadcastLabel(broadcasts []broadcastCandidate) string {

	names := make([]string, 0, len(broadcasts))

	for _, b := range broadcasts {

		if strings.TrimSpace(b.Name) != "" {

			names = append(names, b.Name)

		}

	}

	terms := expandBroadcastLabels(names)

	if len(terms) > 0 {

		return terms[0]

	}

	if len(names) > 0 {

		return names[0]

	}

	return ""

}
