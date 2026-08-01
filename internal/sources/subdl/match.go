package subdl

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	episodeTagRE = regexp.MustCompile(`(?i)(?:^|[.\s_-])s(\d{1,2})e(\d{1,2})(?:[.\s_-]|$)`)
	episodeXRE = regexp.MustCompile(`(?i)(?:^|[.\s_-])(\d{1,2})x(\d{1,2})(?:[.\s_-]|$)`)
	leadingEpisodeRE = regexp.MustCompile(`(?i)^(\d{1,2})\s+`)

	reEpisodeSE = regexp.MustCompile(`(?i)[._\s-]s(\d{1,2})[._\s-]?e(\d{1,3})[._\s-]`)
	reEpisodeXE = regexp.MustCompile(`(?i)[._\s-](\d{1,2})x(\d{1,3})[._\s-]`)
	reEpisodeNumOnly = regexp.MustCompile(`(?i)[._\s-]e(\d{1,3})[._\s-]`)
)

type unpackFile struct {

	URL string `json:"url"`
	Name string `json:"name"`

	ReleaseName string `json:"release_name"`
	Language string `json:"language"`
	Format string `json:"format"`

	Season int `json:"season"`
	Episode int `json:"episode"`

	Hi bool `json:"hi"`

}

type subtitleEntry struct {

	ReleaseName string `json:"release_name"`
	Name string `json:"name"`
	URL string `json:"url"`

	Season int `json:"season"`
	Episode int `json:"episode"`

	Hi bool `json:"hi"`

	UnpackFiles []unpackFile `json:"unpack_files"`

}

// pickTracks selects English candidates the same way Streamly-Web does: episode-aware
// unpack files first, then single-file packs, then season zips as a last resort.
func pickTracks(entries []subtitleEntry, season, episode int) []Release {

	seen := make(map[string]struct{})
	var releases []Release

	add := func(path, name, releaseName, language, format string, hi bool) {

		path = strings.TrimSpace(path)

		if path == "" {

			return

		}

		if _, ok := seen[path]; ok {

			return

		}

		seen[path] = struct{}{}

		releases = append(releases, Release{

			Path: path,
			Name: strings.TrimSpace(name),

			ReleaseName: strings.TrimSpace(releaseName),
			Language: normalizeLanguage(language),
			Format: normalizeFormat(format, name),

			HearingImpaired: hi,

		})

	}

	if season > 0 && episode > 0 {

		for _, entry := range entries {

			for _, file := range entry.UnpackFiles {

				if !fileMatchesEpisode(file, season, episode) {

					continue

				}

				add(file.URL, file.Name, pick(file.ReleaseName, entry.ReleaseName), file.Language, file.Format, file.Hi)

			}

		}

		for _, entry := range entries {

			if !subtitleMatchesEpisode(entry, season, episode) {

				continue

			}

			if len(entry.UnpackFiles) == 1 {

				file := entry.UnpackFiles[0]

				if looksEnglishLanguageTag(file.Language) && !hasForeignLanguageName(file.Name) {

					add(file.URL, file.Name, pick(file.ReleaseName, entry.ReleaseName), file.Language, file.Format, file.Hi)

				}

			}

			if len(entry.UnpackFiles) == 0 {

				path := strings.TrimSpace(entry.URL)

				if path != "" && !isZipPath(path) {

					add(path, pick(entry.Name, entry.ReleaseName), entry.ReleaseName, "en", "", entry.Hi)

				}

			}

		}

		for _, path := range pickSeasonZipPaths(entries, season) {

			add(path, subtitleZipLabel(path), "", "en", "zip", false)

		}

		return releases

	}

	for _, entry := range entries {

		for _, file := range entry.UnpackFiles {

			if !looksEnglishLanguageTag(file.Language) || hasForeignLanguageName(file.Name) {

				continue

			}

			lowerName := strings.ToLower(file.Name)

			if strings.EqualFold(strings.TrimSpace(file.Format), "srt") || strings.HasSuffix(lowerName, ".srt") || strings.HasSuffix(lowerName, ".vtt") {

				add(file.URL, file.Name, pick(file.ReleaseName, entry.ReleaseName), file.Language, file.Format, file.Hi)

			}

		}

		if path := strings.TrimSpace(entry.URL); path != "" {

			add(path, pick(entry.Name, entry.ReleaseName), entry.ReleaseName, "en", "", entry.Hi)

		}

	}

	return releases

}

func normalizeLanguage(language string) string {

	language = strings.ToLower(strings.TrimSpace(language))

	switch language {

	case "en", "eng", "english", "en-us", "en-gb", "en_us", "en_gb", "":

		return "en"

	default:

		return language

	}

}

func normalizeFormat(format, name string) string {

	format = strings.ToLower(strings.TrimSpace(format))

	if format == "srt" || format == "vtt" || format == "zip" {

		return format

	}

	namePath, _, _ := strings.Cut(name, "?")
	lower := strings.ToLower(namePath)

	if strings.HasSuffix(lower, ".vtt") {

		return "vtt"

	}

	if strings.HasSuffix(lower, ".zip") {

		return "zip"

	}

	ext := strings.TrimPrefix(filepath.Ext(lower), ".")

	switch ext {

	case "srt":

		return "srt"

	case "vtt", "webvtt":

		return "vtt"

	case "ass", "ssa":

		return "ass"

	}

	return "srt"

}

func isZipPath(path string) bool {

	before, _, _ := strings.Cut(path, "?")

	return strings.HasSuffix(strings.ToLower(before), ".zip")

}

func subtitleZipLabel(path string) string {

	before, _, _ := strings.Cut(path, "?")
	parts := strings.Split(strings.Trim(before, "/"), "/")

	if len(parts) == 0 {

		return "English"

	}

	return parts[len(parts)-1]

}

func pickSeasonZipPaths(entries []subtitleEntry, season int) []string {

	var preferred []string
	var fallback []string

	for _, entry := range entries {

		if !seasonMatches(entry.Season, season) {

			continue

		}

		path := strings.TrimSpace(entry.URL)

		if path == "" || !isZipPath(path) {

			continue

		}

		joined := strings.ToLower(entry.ReleaseName + " " + entry.Name)

		if strings.Contains(joined, "forced") {

			fallback = append(fallback, path)
			continue

		}

		preferred = append(preferred, path)

	}

	return append(preferred, fallback...)

}

func subtitleMatchesEpisode(entry subtitleEntry, season, episode int) bool {

	if entry.Episode == episode && seasonMatches(entry.Season, season) {

		return true

	}

	for _, label := range []string{entry.ReleaseName, entry.Name} {

		if s, e := parseEpisodeTag(label); e == episode && seasonMatches(s, season) {

			return true

		}

	}

	return false

}

func fileMatchesEpisode(file unpackFile, season, episode int) bool {

	if !looksEnglishLanguageTag(file.Language) || hasForeignLanguageName(file.Name) {

		return false

	}

	for _, label := range []string{file.Name, file.ReleaseName} {

		if s, e := parseEpisodeTag(label); e == episode && seasonMatches(s, season) {

			return true

		}

		if e := parseLeadingEpisode(label); e == episode {

			return true

		}

	}

	if file.Episode == episode && seasonMatches(file.Season, season) {

		return true

	}

	return false

}

func seasonMatches(got, want int) bool {

	return got == 0 || got == want

}

func parseEpisodeTag(label string) (season, episode int) {

	label = strings.TrimSpace(label)

	if label == "" {

		return 0, 0

	}

	if match := episodeTagRE.FindStringSubmatch(label); len(match) == 3 {

		season, _ = strconv.Atoi(match[1])
		episode, _ = strconv.Atoi(match[2])

		return season, episode

	}

	if match := episodeXRE.FindStringSubmatch(label); len(match) == 3 {

		season, _ = strconv.Atoi(match[1])
		episode, _ = strconv.Atoi(match[2])

		return season, episode

	}

	return 0, 0

}

func parseLeadingEpisode(label string) int {

	label = strings.TrimSpace(label)

	if label == "" {

		return 0

	}

	base := label

	if idx := strings.Index(label, "/"); idx >= 0 {

		base = label[idx+1:]

	}

	match := leadingEpisodeRE.FindStringSubmatch(base)

	if len(match) != 2 {

		return 0

	}

	episode, err := strconv.Atoi(match[1])

	if err != nil {

		return 0

	}

	return episode

}

// nameMatchesEpisode is used when unpacking season archives (Streamly-Web helpers.go).
func nameMatchesEpisode(name string, season, episode int) bool {

	padded := " " + name + " "

	if season > 0 {

		if m := reEpisodeSE.FindStringSubmatch(padded); m != nil {

			s, _ := strconv.Atoi(m[1])
			e, _ := strconv.Atoi(m[2])

			return s == season && e == episode

		}

		if m := reEpisodeXE.FindStringSubmatch(padded); m != nil {

			s, _ := strconv.Atoi(m[1])
			e, _ := strconv.Atoi(m[2])

			return s == season && e == episode

		}

		return false

	}

	if m := reEpisodeNumOnly.FindStringSubmatch(padded); m != nil {

		e, _ := strconv.Atoi(m[1])

		return e == episode

	}

	return false

}
