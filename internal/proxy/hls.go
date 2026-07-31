package proxy

import (
	"net/url"
	"regexp"
	"strings"
)

var uriAttribute = regexp.MustCompile(`URI="([^"]+)"`)

func isManifest(target *url.URL, contentType string) bool {

	if strings.Contains(strings.ToLower(contentType), "mpegurl") {

		return true

	}

	return strings.HasSuffix(strings.ToLower(target.Path), ".m3u8")

}

func rewriteManifest(body []byte, base *url.URL, rewrite func(string) string) []byte {

	lines := strings.Split(string(body), "\n")

	for index, line := range lines {

		trimmed := strings.TrimSpace(line)

		if trimmed == "" {

			continue

		}

		if strings.HasPrefix(trimmed, "#") {

			lines[index] = uriAttribute.ReplaceAllStringFunc(line, func(match string) string {

				inner := uriAttribute.FindStringSubmatch(match)[1]

				return `URI="` + rewrite(resolveAgainst(base, inner)) + `"`

			})

			continue

		}

		lines[index] = rewrite(resolveAgainst(base, trimmed))

	}

	return []byte(strings.Join(lines, "\n"))

}

func resolveAgainst(base *url.URL, reference string) string {

	parsed, err := url.Parse(reference)

	if err != nil {

		return reference

	}

	return base.ResolveReference(parsed).String()

}
