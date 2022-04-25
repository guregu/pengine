package pengine

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// Client is a Pengines endpoint.
type Client struct {
	// URL of the pengines server, required.
	URL string
	// Application name, optional. (example: "pengines_sandbox")
	Application string
	// Chunk is the number of query results to accumulate in one response. 1 by default.
	Chunk int

	// SourceText is Prolog source code to load (optional).
	//
	// TODO(guregu): Currently not supported in the Prolog format (use URL instead).
	SourceText string
	// SourceURL specifies a URL of Prolog source for the pengine to load (optional).
	SourceURL string

	// HTTP is the HTTP client used to make API requests.
	// If nil, http.DefaultClient is used.
	HTTP *http.Client

	// If true, prints debug logs.
	Debug bool
}

// Create creates a new pengine. Call Engine's Ask method to query it.
// If destroy is true, the pengine will be automatically destroyed when a query completes.
// If destroy is false, it is the caller's responsibility to destroy the pengine with Engine.Close.
func (c Client) Create(ctx context.Context, destroy bool) (*Engine, error) {
	eng, answer, err := c.create(ctx, "", destroy)
	if err != nil {
		return nil, err
	}
	err = eng.handle(answer)
	return eng, err
}

// Ask creates a new engine with the given initial query and executes it, returning the answers iterator.
func (c Client) Ask(ctx context.Context, query string) (*iterator[Solution], error) {
	eng, answer, err := c.create(ctx, query, true)
	if err != nil {
		return nil, err
	}
	return newIterator[Solution](eng, answer)
}

func (c Client) create(ctx context.Context, query string, destroy bool) (*Engine, answer, error) {
	if c.URL == "" {
		return nil, answer{}, fmt.Errorf("pengine: Server URL not set")
	}

	eng := &Engine{
		client:  c,
		destroy: destroy,
		debug:   c.Debug,
	}
	opts := c.options("json")
	if query != "" {
		opts.Ask = query
	}

	evt, err := eng.post(ctx, "create", opts)
	if err != nil {
		return nil, evt, fmt.Errorf("pengine create error: %w", err)
	}
	return eng, evt, nil
}

func (c Client) client() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

type options struct {
	Format      string `json:"format"`
	Destroy     bool   `json:"destroy"`
	Application string `json:"application,omitempty"`
	Chunk       int    `json:"chunk,omitempty"`
	Ask         string `json:"ask,omitempty"`
	Template    string `json:"template,omitempty"`
	SourceText  string `json:"src_text,omitempty"`
	SourceURL   string `json:"src_url,omitempty"`
}

func (c Client) options(format string) options {
	return options{
		Format:      format,
		Application: c.Application,
		Chunk:       c.Chunk,
		SourceText:  c.SourceText,
		SourceURL:   c.SourceURL,
	}
}

func (opts options) String() string {
	var sb strings.Builder
	first := true
	write := func(strs ...string) {
		if !first {
			sb.WriteRune(',')
		}
		for _, str := range strs {
			sb.WriteString(str)
		}
		first = false
	}
	sb.WriteRune('[')

	if !opts.Destroy {
		write("destroy(false)")
	}

	if opts.Application != "" {
		write("application(", escapeAtom(opts.Application), ")")
	}

	if opts.Chunk > 0 {
		write("chunk(", strconv.Itoa(opts.Chunk), ")")
	}

	if opts.Ask != "" {
		write("ask(", opts.Ask, ")")
	}

	if opts.Template != "" {
		write("template(", opts.Template, ")")
	}

	if opts.SourceURL != "" {
		write("src_url(", escapeAtom(opts.SourceURL), ")")
	}

	sb.WriteRune(']')
	return sb.String()
}
