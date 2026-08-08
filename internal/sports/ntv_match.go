package sports

import (
	"strings"

	"streamly/internal/catalog"
	"streamly/internal/sources/ntv"
)

// Aux channel IDs for NTV-only feeds so they never collide with iptv-org ids.
const ntvAuxPrefix = "ntv:"

// ESPN OTT labels → NTV search terms. Keys match normalizeKey.
var ottCatalogTerms = map[string][]string{

	"apple tv": {"Apple TV", "AppleTV", "Apple TV+"},
	"apple tv+": {"Apple TV", "AppleTV", "Apple TV+"},
	"espn+": {"ESPN+", "ESPN PLUS", "ESPN Plus"},
	"espn unlmtd": {"ESPN+", "ESPN PLUS", "ESPN Plus"},
	"espn unlimited": {"ESPN+", "ESPN PLUS", "ESPN Plus"},
	"peacock": {"Peacock"},
	"amazon": {"Amazon Prime", "Prime Video"},
	"amazon prime": {"Amazon Prime", "Prime Video"},
	"prime video": {"Amazon Prime", "Prime Video"},
	"paramount+": {"Paramount+", "Paramount Plus"},
	"paramount plus": {"Paramount+", "Paramount Plus"},
	"mlbtv": {"MLB.TV", "MLB TV"},
	"nba league pass": {"NBA League Pass", "League Pass"},
	"nhltv": {"NHL.TV", "NHL TV"},
	"max": {"Max", "HBO Max"},
	"netflix": {"Netflix"},
	"disney+": {"Disney+", "Disney Plus"},
	"hulu": {"Hulu"},
	"fubo": {"Fubo", "Fubo Sports"},
	"youtube": {"YouTube"},
	"youtube tv": {"YouTube TV"},

}

func ntvAuxID(ref string) string {

	return ntvAuxPrefix + ref

}

func matchNTVChannel(m Match, listing []ntv.Channel) *MatchedChannel {

	if len(listing) == 0 {

		return nil

	}

	// Team-branded NTV feeds (e.g. "New York Yankees") cover OTT exclusives.
	if m.HomeTeam != nil {

		if ch := findNTVExactName(listing, m.HomeTeam.Name); ch != nil {

			return ch

		}

	}

	if m.AwayTeam != nil {

		if ch := findNTVExactName(listing, m.AwayTeam.Name); ch != nil {

			return ch

		}

	}

	for _, term := range ottSearchTerms(m.Broadcasts) {

		if ch := findNTVLooseName(listing, term); ch != nil {

			return ch

		}

	}

	return nil

}

func ottSearchTerms(broadcasts []string) []string {

	var terms []string
	seen := map[string]bool{}

	add := func(label string) {

		label = strings.TrimSpace(label)

		if label == "" {

			return

		}

		key := normalizeKey(label)

		if seen[key] {

			return

		}

		seen[key] = true
		terms = append(terms, label)

	}

	for _, name := range broadcasts {

		for _, part := range splitBroadcastName(name) {

			pk := normalizeKey(part)

			if labels, ok := ottCatalogTerms[pk]; ok {

				for _, label := range labels {

					add(label)

				}

				continue

			}

			// Also accept the raw OTT label when ESPN names it something we map loosely.
			if skipBroadcasts[pk] {

				add(part)

			}

		}

	}

	return terms

}

func findNTVExactName(listing []ntv.Channel, name string) *MatchedChannel {

	want := normalizeKey(name)

	if want == "" {

		return nil

	}

	for i := range listing {

		ch := &listing[i]

		if normalizeKey(ch.Name) == want {

			return matchedFromNTV(ch)

		}

	}

	// "Athletics" vs "Oakland Athletics" / trailing market tokens.
	for i := range listing {

		ch := &listing[i]
		cn := normalizeKey(ch.Name)

		if cn == "" {

			continue

		}

		if strings.HasSuffix(cn, " "+want) || strings.HasPrefix(cn, want+" ") {

			return matchedFromNTV(ch)

		}

	}

	return nil

}

func findNTVLooseName(listing []ntv.Channel, name string) *MatchedChannel {

	want := normalizeKey(name)

	if want == "" || len(want) < 3 {

		return nil

	}

	var best *ntv.Channel
	bestScore := 0

	for i := range listing {

		ch := &listing[i]
		cn := normalizeKey(ch.Name)
		score := ntvNameScore(want, cn)

		if score > bestScore {

			bestScore = score
			best = ch

		}

	}

	if best == nil || bestScore < 70 {

		return nil

	}

	return matchedFromNTV(best)

}

func ntvNameScore(want, have string) int {

	if want == "" || have == "" {

		return 0

	}

	if want == have {

		return 100

	}

	// Prefer unnumbered brand feeds ("ESPN+ USA") over "ESPN PLUS 47".
	if strings.HasPrefix(have, want) {

		rest := strings.TrimSpace(have[len(want):])

		if rest == "" {

			return 100

		}

		if rest == "usa" || rest == "us" || rest == "uk" || rest == "gb" {

			return 95

		}

		// Numbered sibling: still usable but ranked lower.
		if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {

			return 80

		}

		if strings.HasPrefix(rest, "usa") || strings.HasPrefix(rest, "us ") {

			return 90

		}

		return 75

	}

	if strings.Contains(have, want) && len(want) >= 5 {

		return 70

	}

	return 0

}

func matchedFromNTV(ch *ntv.Channel) *MatchedChannel {

	if ch == nil || ch.ID == "" {

		return nil

	}

	return &MatchedChannel{

		ID: ntvAuxID(ch.ID),
		Name: ch.Name,

	}

}

func ensureNTVAux(cat *catalog.Catalog, listing []ntv.Channel, matched *MatchedChannel) {

	if cat == nil || matched == nil || !strings.HasPrefix(matched.ID, ntvAuxPrefix) {

		return

	}

	ref := strings.TrimPrefix(matched.ID, ntvAuxPrefix)

	for i := range listing {

		ch := &listing[i]

		if ch.ID != ref {

			continue

		}

		cat.EnsureAux(catalog.Channel{

			ID: matched.ID,
			Name: ch.Name,

			Country: strings.ToUpper(ch.Code),

			Sources: []catalog.Source{{

				Provider: catalog.ProviderNTV,
				Ref: ch.ID,

			}},

		})

		return

	}

}
