package catalog

import (
	"regexp"
	"strings"
)

// Taken from the trailing words that actually occur in provider titles rather than from a general country list.
var countryTokens = map[string]string{

	"usa": "US", "us": "US",
	"uk": "UK", "gb": "UK",
	"ca": "CA", "canada": "CA",
	"mx": "MX", "mexico": "MX",
	"nl": "NL", "netherlands": "NL", "netherland": "NL",
	"de": "DE", "germany": "DE",
	"fr": "FR", "france": "FR",
	"es": "ES", "spain": "ES", "espana": "ES",
	"it": "IT", "italy": "IT",
	"pt": "PT", "portugal": "PT",
	"br": "BR", "brasil": "BR", "brazil": "BR",
	"ar": "AR", "argentina": "AR",
	"tr": "TR", "turkey": "TR",
	"au": "AU", "australia": "AU",
	"nz": "NZ",
	"qa": "QA", "qatar": "QA",
	"my": "MY", "malaysia": "MY",
	"ie": "IE", "ireland": "IE",
	"pl": "PL", "poland": "PL",
	"ro": "RO", "romania": "RO",
	"gr": "GR", "greece": "GR",
	"se": "SE", "sweden": "SE",
	"dk": "DK", "denmark": "DK",
	"no": "NO", "norway": "NO",
	"fi": "FI", "finland": "FI",
	"cz": "CZ", "sk": "SK",
	"bg": "BG", "bulgaria": "BG",
	"rs": "RS", "serbia": "RS",
	"hr": "HR", "croatia": "HR",
	"bih": "BA",
	"hu": "HU", "hungary": "HU",
	"cy": "CY", "cyprus": "CY",
	"ru": "RU", "russia": "RU",
	"il": "IL", "israel": "IL",
	"ae": "AE", "uae": "AE",
	"sa": "SA", "pk": "PK",
	"za": "ZA", "jp": "JP", "kr": "KR",

}

// The one guess in the matcher. It breaks a tie only between entries that are the same channel carried in several
// countries, and only when the title named none — DaddyLive tags everything except its UK and US feeds.
var preferredCountries = []string{"UK", "US"}

var parenthetical = regexp.MustCompile(`\(([^)]*)\)`)

// Local affiliates carry their network as an alt name, so a primary-name hit has to outrank an alt-name hit.
type index struct {

	primaryExact map[string][]reference
	primaryLoose map[string][]reference

	altExact map[string][]reference
	altLoose map[string][]reference

}

func newIndex(references []reference) *index {

	built := &index{

		primaryExact: map[string][]reference{},
		primaryLoose: map[string][]reference{},

		altExact: map[string][]reference{},
		altLoose: map[string][]reference{},

	}

	for _, entry := range references {

		add(built.primaryExact, exact(entry.Name), entry)
		add(built.primaryLoose, normalise(entry.Name), entry)

		for _, name := range entry.AltNames {

			add(built.altExact, exact(name), entry)
			add(built.altLoose, normalise(name), entry)

		}

	}

	return built

}

func add(into map[string][]reference, key string, entry reference) {

	if key == "" {

		return

	}

	into[key] = append(into[key], entry)

}

type match struct {

	reference reference

	// True when the country was assumed rather than read off the title, so the count can be reported.
	assumed bool

}

// A wrong match shows the wrong channel, so anything still ambiguous after ranking is refused rather than guessed (§5.2).
func (i *index) match(title string) (match, bool) {

	for _, candidate := range variantsOf(title) {

		if found, ok := i.lookup(candidate); ok {

			return found, true

		}

	}

	return match{}, false

}

func (i *index) lookup(candidate variant) (match, bool) {

	tiers := []struct {

		table map[string][]reference
		key string

	}{

		{i.primaryExact, exact(candidate.name)},
		{i.primaryLoose, normalise(candidate.name)},
		{i.altExact, exact(candidate.name)},
		{i.altLoose, normalise(candidate.name)},

	}

	for _, tier := range tiers {

		if found, ok := only(tier.table[tier.key], candidate.country); ok {

			return found, true

		}

	}

	return match{}, false

}

func only(candidates []reference, country string) (match, bool) {

	unique := dedupe(candidates)

	if country != "" {

		var matched []reference

		for _, candidate := range unique {

			if sameCountry(candidate.Country, country) {

				matched = append(matched, candidate)

			}

		}

		if len(matched) == 1 {

			return match{reference: matched[0]}, true

		}

		return match{}, false

	}

	if len(unique) == 1 {

		return match{reference: unique[0]}, true

	}

	// Different channels that merely collide on a name stay refused; only one channel in several countries is guessable.
	if !sameName(unique) {

		return match{}, false

	}

	for _, preferred := range preferredCountries {

		for _, candidate := range unique {

			if sameCountry(candidate.Country, preferred) {

				return match{reference: candidate, assumed: true}, true

			}

		}

	}

	return match{}, false

}

func dedupe(candidates []reference) []reference {

	unique := make([]reference, 0, len(candidates))

	seen := map[string]bool{}

	for _, candidate := range candidates {

		if seen[candidate.ID] {

			continue

		}

		seen[candidate.ID] = true

		unique = append(unique, candidate)

	}

	return unique

}

func sameName(candidates []reference) bool {

	for _, candidate := range candidates {

		if exact(candidate.Name) != exact(candidates[0].Name) {

			return false

		}

	}

	return true

}

type variant struct {

	name string
	country string

}

// The country-qualified reading is tried first because most provider titles name one, and a failed strip simply falls through.
func variantsOf(title string) []variant {

	lower := strings.ToLower(strings.TrimSpace(title))

	forms := []string{strings.TrimSpace(parenthetical.ReplaceAllString(lower, "")), lower}

	if match := parenthetical.FindStringSubmatch(lower); match != nil {

		forms = append(forms, strings.TrimSpace(match[1]))

	}

	variants := []variant{}

	for _, form := range forms {

		if form == "" {

			continue

		}

		if trimmed, country := splitCountry(form); country != "" {

			variants = append(variants, variant{name: trimmed, country: country})

		}

		variants = append(variants, variant{name: form})

	}

	return variants

}

func splitCountry(name string) (string, string) {

	fields := strings.Fields(name)

	if len(fields) < 2 {

		return name, ""

	}

	country, ok := countryTokens[fields[len(fields)-1]]

	if !ok {

		return name, ""

	}

	return strings.Join(fields[:len(fields)-1], " "), country

}

// iptv-org labels the United Kingdom "UK" rather than the ISO "GB"; accepting both survives either convention.
func sameCountry(a string, b string) bool {

	if a == b {

		return true

	}

	return (a == "UK" || a == "GB") && (b == "UK" || b == "GB")

}

func exact(name string) string {

	return strings.Join(strings.Fields(strings.ToLower(name)), " ")

}

func normalise(name string) string {

	var builder strings.Builder

	for _, r := range strings.ToLower(name) {

		if ('a' <= r && r <= 'z') || ('0' <= r && r <= '9') {

			builder.WriteRune(r)

		}

	}

	return builder.String()

}
