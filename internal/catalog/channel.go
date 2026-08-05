package catalog

const (

	ProviderDaddyLive = "daddylive"
	ProviderNTV = "ntv"

)

// SourcePriority is the resolve order when a channel has multiple providers.
// Earlier entries are tried first. Append new providers here when wiring them in.
var SourcePriority = []string{

	ProviderDaddyLive,
	ProviderNTV,

}

type Source struct {

	Provider string `json:"provider"`
	Ref string `json:"ref"`

}

func providerRank(provider string) int {

	for index, name := range SourcePriority {

		if name == provider {

			return index

		}

	}

	return len(SourcePriority)

}

// sortSources orders by SourcePriority so failover walks primary → backup → …
func sortSources(sources []Source) {

	if len(sources) < 2 {

		return

	}

	for i := 1; i < len(sources); i++ {

		current := sources[i]
		j := i - 1

		for j >= 0 && providerRank(sources[j].Provider) > providerRank(current.Provider) {

			sources[j+1] = sources[j]
			j--

		}

		sources[j+1] = current

	}

}

func hasProvider(sources []Source, provider string) bool {

	for _, source := range sources {

		if source.Provider == provider {

			return true

		}

	}

	return false

}

// Identity and metadata come from iptv-org; sources are backfilled onto it (see _docs/DESIGN.md §5.2).
type Channel struct {

	ID string `json:"id"`
	Name string `json:"name"`

	Country string `json:"country,omitempty"`
	Network string `json:"network,omitempty"`

	Categories []string `json:"categories,omitempty"`

	Logo string `json:"logo,omitempty"`

	Sources []Source `json:"sources"`

}
