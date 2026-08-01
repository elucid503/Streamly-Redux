package daddylive

import (
	"context"
	"html"
	"regexp"
	"strings"
)

// Cards carry the id and a normalised title as attributes, which is all the catalog needs to match them.
var cardPattern = regexp.MustCompile(`(?is)href="[^"]*watch\.php\?id=(\d+)"[^>]*?data-title="([^"]*)"`)

type Channel struct {

	Ref string
	Name string

}

func (c *Client) Channels(ctx context.Context) ([]Channel, error) {

	body, err := c.fetch(ctx, baseURL+"/24-7-channels.php", baseURL+"/")

	if err != nil {

		return nil, err

	}

	matches := cardPattern.FindAllStringSubmatch(body, -1)

	channels := make([]Channel, 0, len(matches))

	seen := map[string]bool{}

	for _, match := range matches {

		ref := match[1]

		if seen[ref] {

			continue

		}

		seen[ref] = true

		name := strings.TrimSpace(html.UnescapeString(match[2]))

		if name == "" {

			continue

		}

		channels = append(channels, Channel{

			Ref: ref,
			Name: name,

		})

	}

	return channels, nil

}
