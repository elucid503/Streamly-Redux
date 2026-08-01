package resolve

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"streamly/internal/sources/febbox"
)

// The pane is far too small for 4K to be visible and every byte relays through Discord's proxy (see _docs/DESIGN.md §4.7).
const maxDefaultHeight = 1080

var (

	videoExtensions = []string{".mp4", ".mkv", ".avi", ".mov", ".m4v"}

	heightPattern = regexp.MustCompile(`(\d{3,4})\s*[pP]`)
	seasonPattern = regexp.MustCompile(`(?i)(?:season|s)\s*0*(\d{1,2})`)
	episodePattern = regexp.MustCompile(`(?i)s\s*0*(\d{1,2})\s*[\s._-]*e\s*0*(\d{1,3})`)
	loneEpisodePattern = regexp.MustCompile(`(?i)(?:^|[\s._-])e(?:p|pisode)?\s*0*(\d{1,3})(?:[\s._-]|$)`)

)

func pickVideo(files []febbox.File) (*febbox.File, error) {

	for index, file := range files {

		if !file.IsDir && isVideo(file.Name) {

			return &files[index], nil

		}

	}

	return nil, ErrNoPlayableFile

}

func pickEpisode(files []febbox.File, season int, episode int) *febbox.File {

	for index, file := range files {

		if file.IsDir || !isVideo(file.Name) {

			continue

		}

		if match := episodePattern.FindStringSubmatch(file.Name); match != nil {

			if number(match[1]) == season && number(match[2]) == episode {

				return &files[index]

			}

			continue

		}

		if match := loneEpisodePattern.FindStringSubmatch(file.Name); match != nil && number(match[1]) == episode {

			return &files[index]

		}

	}

	return nil

}

func matchesSeason(name string, season int) bool {

	match := seasonPattern.FindStringSubmatch(name)

	if match == nil {

		return season <= 1

	}

	return number(match[1]) == season

}

func isVideo(name string) bool {

	lower := strings.ToLower(name)

	for _, extension := range videoExtensions {

		if strings.HasSuffix(lower, extension) {

			return true

		}

	}

	return false

}

func sortByHeight(renditions []Quality) {

	sort.SliceStable(renditions, func(a int, b int) bool {

		return renditions[a].Height > renditions[b].Height

	})

}

func defaultRendition(renditions []Quality) *Quality {

	for index := range renditions {

		if renditions[index].Height <= maxDefaultHeight {

			return &renditions[index]

		}

	}

	if len(renditions) == 0 {

		return nil

	}

	return &renditions[len(renditions)-1]

}

func heightOf(label string) int {

	if match := heightPattern.FindStringSubmatch(label); match != nil {

		return number(match[1])

	}

	if strings.Contains(strings.ToUpper(label), "4K") {

		return 2160

	}

	return 0

}

func isOriginalLabel(label string) bool {

	switch strings.ToUpper(strings.TrimSpace(label)) {

	case "ORG", "ORIGINAL", "SOURCE":

		return true

	}

	return false

}

func number(text string) int {

	value, err := strconv.Atoi(text)

	if err != nil {

		return 0

	}

	return value

}

func EpisodeCaption(season int, episode int, title string) string {

	caption := fmt.Sprintf("S%d · E%d", season, episode)

	if title != "" {

		caption += " — " + title

	}

	return caption

}
