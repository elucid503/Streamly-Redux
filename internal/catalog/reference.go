package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (

	iptvChannelsURL = "https://iptv-org.github.io/api/channels.json"
	iptvLogosURL = "https://iptv-org.github.io/api/logos.json"

)

const maxBodyBytes = 64 << 20

// One canonical channel as iptv-org describes it. Providers are matched onto these, never to each other.
type reference struct {

	ID string
	Name string

	AltNames []string

	Country string
	Network string

	Categories []string

	Logo string

}

func fetchReference(ctx context.Context, client *http.Client) ([]reference, error) {

	var raw []struct {

		ID string `json:"id"`
		Name string `json:"name"`

		AltNames []string `json:"alt_names"`

		Country string `json:"country"`
		Network string `json:"network"`

		Categories []string `json:"categories"`

		IsNSFW bool `json:"is_nsfw"`
		Closed string `json:"closed"`

	}

	if err := fetchJSON(ctx, client, iptvChannelsURL, &raw); err != nil {

		return nil, fmt.Errorf("catalog: iptv-org channels unavailable: %w", err)

	}

	var logos []struct {

		Channel string `json:"channel"`
		InUse bool `json:"in_use"`

		URL string `json:"url"`

	}

	if err := fetchJSON(ctx, client, iptvLogosURL, &logos); err != nil {

		return nil, fmt.Errorf("catalog: iptv-org logos unavailable: %w", err)

	}

	byChannel := map[string]string{}

	for _, logo := range logos {

		if logo.URL == "" {

			continue

		}

		if existing, taken := byChannel[logo.Channel]; taken && !logo.InUse && existing != "" {

			continue

		}

		byChannel[logo.Channel] = logo.URL

	}

	references := make([]reference, 0, len(raw))

	for _, entry := range raw {

		// A closed channel still has a name that would happily match a live provider title.
		if entry.IsNSFW || entry.Closed != "" {

			continue

		}

		references = append(references, reference{

			ID: entry.ID,
			Name: entry.Name,

			AltNames: entry.AltNames,

			Country: entry.Country,
			Network: entry.Network,

			Categories: entry.Categories,

			Logo: byChannel[entry.ID],

		})

	}

	return references, nil

}

func fetchJSON(ctx context.Context, client *http.Client, target string, out any) error {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)

	if err != nil {

		return err

	}

	resp, err := client.Do(req)

	if err != nil {

		return err

	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {

		return fmt.Errorf("%s returned %s", target, resp.Status)

	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))

	if err != nil {

		return err

	}

	return json.Unmarshal(body, out)

}

func referenceClient() *http.Client {

	return &http.Client{

		Timeout: 60 * time.Second,

	}

}
