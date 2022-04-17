package pengine

import (
	"context"
	"encoding/json"
	"time"
)

// Answers is an iterator of query results.
// Use Next to prepare a result of type T and then Current to obtain it.
// Make sure to check Err after you finish iterating.
// Err will return ErrFailed if the query failed at least once without succeeding at least once.
//
//	answers, err := client.Ask(ctx, "between(1,6,X)")
//	if err != nil {
//		panic(err)
//	}
//	var got []json.Number
//	for as.Next() {
//		cur := as.Current()
//		x := cur["X"]
//		got = append(got, x.Number)
//	}
//	// got = {"1", "2", "3", ...}
//	if err := answers.Error(); err != nil {
//		panic(err)
//	}
type Answers[T any] interface {
	// Next prepares the next query result and returns true if there is a result.
	Next(context.Context) bool
	// Current returns the current query result.
	Current() T
	// Close kills this query (in pengine terms, stops it).
	// It is not necessary to call Close if all results were iterated through.
	Close() error
	// Cumulative returns the cumulative time taken by this query, as reported by pengines.
	Cumulative() time.Duration
	// Engine returns this query's underlying Engine.
	Engine() *Engine
	// Err returns the error encountered by this query.
	// This should always be checked after iteration finishes.
	// Returns ErrFailed if the query failed at least once without succeeding at least once.
	Err() error
}

// answer is a pengines JSON format API response.
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

// iterator is an iterator for query results.
type iterator[T any] struct {
	eng  *Engine
	buf  []T
	cur  T
	more bool
	good int     // count of successes
	bad  int     // count of failures
	cum  float64 // cumulative time taken
	err  error
}

func (as *iterator[T]) Engine() *Engine {
	return as.eng
}

func newIterator[T any](e *Engine, a answer) (*iterator[T], error) {
	as := &iterator[T]{
		eng: e,
	}
	err := as.handle(a)
	return as, err
}

func (as *iterator[T]) handle(a answer) error {
	if err := as.eng.handle(a); err != nil {
		return err
	}

	switch a.Event {
	case "success":
		var data []T
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
		defer as.eng.die()
		as.err = ErrDead
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

func (as *iterator[T]) Next(ctx context.Context) bool {
more:
	switch {
	case as.err != nil:
		return false
	case ctx.Err() != nil:
		as.err = ctx.Err()
		return false
	case len(as.buf) > 0:
		as.cur = as.pop()
		return true
	case as.more:
		a, err := as.eng.send(ctx, "next")
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
func (as *iterator[T]) Current() T {
	return as.cur
}

// Close stops this query. It is not necessary to call this if all results have been iterated through.
func (as *iterator[T]) Close() error {
	if as.eng.dead {
		return nil
	}
	a, err := as.eng.send(context.Background(), "stop")
	if err != nil {
		return err
	}
	return as.handle(a)
}

// Error returns an error encountered by this query, if any.
func (as *iterator[T]) Err() error {
	if as.err == nil && as.bad > 0 && as.good == 0 {
		return ErrFailed
	}
	return as.err
}

// Cumulative returns the cumulative time taken by this query, as reported by pengines.
func (as *iterator[T]) Cumulative() time.Duration {
	return time.Duration(float64(time.Second) * as.cum)
}

func (as *iterator[T]) pop() T {
	data := as.buf[0]
	as.buf = as.buf[1:]
	return data
}
