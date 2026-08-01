package catalog

const (

	ProviderDaddyLive = "daddylive"
	ProviderNTV = "ntv"

)

type Source struct {

	Provider string `json:"provider"`
	Ref string `json:"ref"`

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
