package pengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	// ErrDead is an error returned when the pengine has died.
	ErrDead = fmt.Errorf("pengine: died")
	// ErrFailed is an error returned when a query failed (returned no results).
	ErrFailed = fmt.Errorf("pengine: query failed")
)

// Server is a Pengines endpoint.
type Server struct {
	// URL of the pengines server, required.
	URL string
	// Application name, optional. (example: "pengines_sandbox")
	Application string
	// Chunk is the number of query results to accumulate in one response. 1 by default.
	Chunk int
	// Client is the HTTP client used to make API requests.
	// If nil, http.DefaultClient is used.
	Client *http.Client
}

// Create creates a new engine. Call Engine's Ask method to query it.
// TODO(guregu): currently this is not very useful, need to provide option not to auto-destroy.
func (srv Server) Create() (*Engine, error) {
	eng, answer, err := srv.create("")
	if err != nil {
		return nil, err
	}
	err = eng.handle(answer)
	return eng, err
}

// Ask creates a new engine with the given initial query and executes it, returning the answers iterator.
func (srv Server) Ask(query string) (*Answers, error) {
	eng, answer, err := srv.create(query)
	if err != nil {
		return nil, err
	}
	return newAnswers(eng, answer)
}

func (srv Server) create(query string) (*Engine, answer, error) {
	if srv.URL == "" {
		return nil, answer{}, fmt.Errorf("pengine: Server URL not set")
	}

	chunk := srv.Chunk
	if chunk == 0 {
		chunk = 1
	}

	eng := &Engine{
		server: srv,
		chunk:  chunk,
	}
	opts := struct {
		Format      string `json:"format"`
		Application string `json:"application,omitempty"`
		Chunk       int    `json:"chunk"`
		Ask         string `json:"ask,omitempty"`
	}{
		Format: "json",
		Chunk:  chunk,
	}
	if query != "" {
		opts.Ask = query
	}

	evt, err := eng.post("create", opts)
	if err != nil {
		return nil, evt, fmt.Errorf("pengine create error: %w", err)
	}
	return eng, evt, nil
}

func (srv Server) client() *http.Client {
	if srv.Client != nil {
		return srv.Client
	}
	return http.DefaultClient
}

// Engine is a pengine.
type Engine struct {
	id        string
	server    Server
	openLimit int // TODO: use this
	chunk     int
	dead      bool
}

// ID return this pengine's ID.
func (e *Engine) ID() string {
	return e.id
}

// answer is a pengines API response.
type answer struct {
	Data       json.RawMessage `json:"data"`
	Event      string          `json:"event"`
	ID         string          `json:"id"`
	More       bool            `json:"more"`
	Projection []string        `json:"projection"`
	Time       float64         `json:"time"` // time taken
	Code       string          `json:"code"` // error code
	OpenLimit  int             `json:"slave_limit"`
	Answer     *answer         `json:"answer"`
}

// Ask queries the pengine, returning an answers iterator.
func (e *Engine) Ask(query string) (*Answers, error) {
	if e.dead {
		return nil, ErrDead
	}

	query = fmt.Sprintf("ask((%s), [chunk(%d)])", query, e.chunk)
	answer, err := e.send(query)
	if err != nil {
		return nil, fmt.Errorf("pengine ask error: %w", err)
	}
	return newAnswers(e, answer)
}

func (e *Engine) die() {
	e.dead = true
}

// Answers is an iterator for query results.
type Answers struct {
	eng  *Engine
	buf  []Solution
	cur  Solution
	more bool
	good int     // count of successes
	bad  int     // count of failures
	cum  float64 // cumulative time taken
	err  error
}

func newAnswers(e *Engine, a answer) (*Answers, error) {
	as := &Answers{
		eng: e,
	}
	err := as.handle(a)
	return as, err
}

func (as *Answers) handle(a answer) error {
	if err := as.eng.handle(a); err != nil {
		return err
	}

	switch a.Event {
	case "success":
		var data []Solution
		if err := json.Unmarshal(a.Data, &data); err != nil {
			return err
		}
		as.buf = append(as.buf, data...)
		as.good += len(data)
		as.more = a.More
		as.cum += a.Time
	case "failure":
		as.bad++
		as.cum += a.Time
	case "destroy":
		defer as.eng.die()
		if len(a.Data) == 0 {
			break
		}
		var child answer
		if err := json.Unmarshal(a.Data, &child); err != nil {
			return err
		}
		if err := as.handle(child); err != nil {
			return err
		}
	case "stop":
		defer as.eng.die()
	case "die":
		as.err = ErrDead
		defer as.eng.die()
	case "error":
		var msg string
		if err := json.Unmarshal(a.Data, &msg); err != nil {
			return err
		}
		as.err = Error{Code: a.Code, Data: msg}
	}

	if a.Answer != nil {
		return as.handle(*a.Answer)
	}
	return nil
}

// Next fetches the next result and returns true if successful. Use Current to grab the result.
// Make sure to check Error after you finish iterating.
//
//	 answers, err := srv.Ask("between(1,6,X)")
//	 if err != nil {
//	 	panic(err)
//	 }
//	 var got []engine.Term
//	 for as.Next() {
//	 	cur := as.Current()
//	 	x := cur["X"]
//	 	got = append(got, x.Prolog())
//	 }
//	 // got = []engine.Term{engine.Integer(1), engine.Integer(2), ...}
//	 if err := answers.Error(); err != nil {
//	 	panic(err)
//	 }
func (as *Answers) Next() bool {
more:
	switch {
	case as.err != nil:
		return false
	case len(as.buf) > 0:
		as.cur = as.pop()
		return true
	case as.more:
		a, err := as.eng.send(fmt.Sprintf("next(%d)", as.eng.chunk))
		if err != nil {
			as.err = err
			return false
		}
		if err := as.handle(a); err != nil {
			as.err = err
			return false
		}
		goto more
	}
	return false
}

// Current returns the current Solution.
func (as *Answers) Current() Solution {
	return as.cur
}

// Close stops this query. It is not necessary to call this if all results have been iterated through.
func (as *Answers) Close() error {
	if as.eng.dead {
		return nil
	}
	a, err := as.eng.send("stop")
	if err != nil {
		return err
	}
	return as.handle(a)
}

// Error returns an error encountered by this query, if any.
func (as *Answers) Error() error {
	if as.err == nil && as.bad > 0 && as.good == 0 {
		return ErrFailed
	}
	return as.err
}

// Cumulative returns the cumulative time taken by this query, as reported by pengines.
func (as *Answers) Cumulative() time.Duration {
	return time.Duration(float64(time.Second) * as.cum)
}

func (as *Answers) pop() Solution {
	data := as.buf[0]
	as.buf = as.buf[1:]
	return data
}

func (e *Engine) handle(a answer) error {
	switch a.Event {
	case "create":
		e.id = a.ID
		e.openLimit = a.OpenLimit
	case "destroy":
		e.die()
	}
	if a.Answer != nil {
		return e.handle(*a.Answer)
	}
	return nil
}

// Close destroys this engine. It is usually not necessary to do this as pengines will destroy themselves.
func (e *Engine) Close() error {
	if e.dead {
		return nil
	}
	a, err := e.send("destroy")
	if err != nil {
		return err
	}
	return e.handle(a)
}

// Error is an error from the pengines API.
type Error struct {
	Code string
	Data string
}

// Error implements the error interface.
func (err Error) Error() string {
	return fmt.Sprintf("pengine: %s: %s", err.Code, err.Data)
}

func (e *Engine) post(action string, body any) (answer, error) {
	var v answer
	var r io.Reader
	if body != nil {
		bs, err := json.Marshal(body)
		if err != nil {
			return v, err
		}
		r = bytes.NewReader(bs)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf(e.server.URL+"/pengine/%s", action), r)
	if err != nil {
		return v, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := e.server.client().Do(req)
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&v)
	return v, err
}

func (e *Engine) send(body string) (answer, error) {
	var v answer
	r := strings.NewReader(body + "\n.")

	href := fmt.Sprintf("%s/pengine/send?format=json&id=%s", e.server.URL, url.QueryEscape(e.id))
	req, err := http.NewRequest("POST", href, r)
	if err != nil {
		return v, err
	}
	req.Header.Set("Content-Type", "application/x-prolog; charset=utf-8")

	resp, err := e.server.client().Do(req)
	if err != nil {
		return v, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return v, fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&v)
	return v, err
}
