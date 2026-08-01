package febbox

import (
	"html"
	"regexp"
	"strings"
)

// video_quality_list returns markup rather than fields, so the renditions are read off attributes and class text.
var (

	urlAttribute = regexp.MustCompile(`data-url=["']([^"']+)["']`)
	qualityAttribute = regexp.MustCompile(`data-quality=["']([^"']+)["']`)

	nameText = regexp.MustCompile(`(?is)class=["'][^"']*\bname\b[^"']*["'][^>]*>(.*?)<`)
	sizeText = regexp.MustCompile(`(?is)class=["'][^"']*\bsize\b[^"']*["'][^>]*>(.*?)<`)

)

func parseQualities(markup string) []Quality {

	chunks := splitOnAttribute(markup, "data-url=")

	qualities := make([]Quality, 0, len(chunks))

	for _, chunk := range chunks {

		target := firstGroup(urlAttribute, chunk)

		if target == "" || !strings.HasPrefix(target, "http") {

			continue

		}

		qualities = append(qualities, Quality{

			URL: target,
			Quality: firstGroup(qualityAttribute, chunk),

			Name: firstGroup(nameText, chunk),
			Size: firstGroup(sizeText, chunk),

		})

	}

	return qualities

}

func splitOnAttribute(markup string, attribute string) []string {

	parts := strings.Split(markup, attribute)

	if len(parts) < 2 {

		return nil

	}

	chunks := make([]string, 0, len(parts)-1)

	for _, part := range parts[1:] {

		chunks = append(chunks, attribute+part)

	}

	return chunks

}

func firstGroup(pattern *regexp.Regexp, text string) string {

	match := pattern.FindStringSubmatch(text)

	if match == nil {

		return ""

	}

	return strings.TrimSpace(html.UnescapeString(match[1]))

}
