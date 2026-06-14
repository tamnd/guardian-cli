package guardian

import (
	"context"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes guardian as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/guardian-cli/guardian"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// guardian:// URIs by routing to the operations Register installs.
func init() { kit.Register(Domain{}) }

// Domain is the guardian driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "guardian",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "guardian",
			Short:  "A command line for The Guardian newspaper.",
			Long: `A command line for The Guardian newspaper.

guardian reads public Guardian content via the open API (api-key=test),
shapes it into clean records, and prints output that pipes into the rest of
your tools. No account required — the test key is officially supported.`,
			Site: "www.theguardian.com",
			Repo: "https://github.com/tamnd/guardian-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "search",
		Group:   "read",
		List:    true,
		Summary: "Search Guardian articles",
		Args:    []kit.Arg{{Name: "query", Help: "search query"}},
	}, searchArticles)

	kit.Handle(app, kit.OpMeta{
		Name:    "sections",
		Group:   "read",
		List:    true,
		Summary: "List all Guardian sections",
	}, listSections)

	kit.Handle(app, kit.OpMeta{
		Name:    "tags",
		Group:   "read",
		List:    true,
		Summary: "Browse Guardian tags",
	}, listTags)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	dcfg := DefaultConfig()
	if cfg.UserAgent != "" {
		dcfg.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		dcfg.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		dcfg.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		dcfg.Timeout = cfg.Timeout
	}
	return NewClientWithConfig(dcfg), nil
}

// --- inputs ---

type searchInput struct {
	Query   string  `kit:"arg"          help:"search query"`
	Section string  `kit:"flag"         help:"section filter (technology, science, politics, sport, culture, world)"`
	From    string  `kit:"flag"         help:"from date (YYYY-MM-DD)"`
	To      string  `kit:"flag"         help:"to date (YYYY-MM-DD)"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

type sectionsInput struct {
	Client *Client `kit:"inject"`
}

type tagsInput struct {
	Section string  `kit:"flag"         help:"filter tags by section"`
	Limit   int     `kit:"flag,inherit" help:"max results"`
	Client  *Client `kit:"inject"`
}

// --- handlers ---

func searchArticles(ctx context.Context, in searchInput, emit func(*Article) error) error {
	articles, err := in.Client.SearchArticles(ctx, in.Query, in.Section, in.From, in.To, in.Limit)
	if err != nil {
		return err
	}
	for i := range articles {
		if err := emit(&articles[i]); err != nil {
			return err
		}
	}
	return nil
}

func listSections(ctx context.Context, in sectionsInput, emit func(*Section) error) error {
	sections, err := in.Client.ListSections(ctx)
	if err != nil {
		return err
	}
	for i := range sections {
		if err := emit(&sections[i]); err != nil {
			return err
		}
	}
	return nil
}

func listTags(ctx context.Context, in tagsInput, emit func(*Tag) error) error {
	tags, err := in.Client.ListTags(ctx, in.Section, in.Limit)
	if err != nil {
		return err
	}
	for i := range tags {
		if err := emit(&tags[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver: pure string functions, network-free ---

// Classify turns any accepted input into a canonical (type, id).
// - "technology/2024/abc" (contains "/") → ("article", id)
// - "technology" (single token) → ("section", id)
// - "climate change" (contains space) → ("query", q)
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty guardian reference")
	}
	// Strip known guardian web URL prefix
	if u, e := url.Parse(input); e == nil && (u.Scheme == "http" || u.Scheme == "https") {
		input = strings.Trim(u.Path, "/")
	}
	if strings.Contains(input, "/") {
		return "article", input, nil
	}
	if strings.Contains(input, " ") {
		return "query", input, nil
	}
	return "section", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "article", "section":
		return "https://www.theguardian.com/" + strings.Trim(id, "/"), nil
	case "query":
		return "https://www.theguardian.com/search?q=" + url.QueryEscape(id), nil
	default:
		return "", errs.Usage("guardian has no resource type %q", uriType)
	}
}
