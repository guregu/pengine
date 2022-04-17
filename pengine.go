package pengine

import (
	"context"
	"fmt"

	"github.com/ichiban/prolog/engine"
)

var (
	// ErrDead is an error returned when the pengine has died.
	ErrDead = fmt.Errorf("pengine: died")
	// ErrFailed is an error returned when a query failed (returned no results).
	ErrFailed = fmt.Errorf("pengine: query failed")
)

// Ask creates a new pengine with the given initial query, executing it and returning an answers iterator.
// Queries must be in Prolog syntax without the terminating period or linebreak, such as:
//
//	between(1,3,X)
//
// This uses the JSON format, so T can be anything that can unmarshal from the pengine result data.
// This package provides a Solutions type that can handle most results in a general manner.
func Ask[T any](ctx context.Context, c Client, query string) (Answers[T], error) {
	eng, answer, err := c.create(ctx, query, true)
	if err != nil {
		return nil, err
	}
	return newIterator[T](eng, answer)
}

// AskProlog creates a new pengine with the given initial query, executing it and returning an answers iterator.
// Queries must be in Prolog syntax without the terminating period or linebreak, such as:
//
//	between(1,3,X)
//
// This uses the Prolog format and answers are ichiban/prolog terms.
// Because ichiban/prolog is used to interpret results, using SWI's nonstandard syntax extensions like dictionaries may break it.
func AskProlog(ctx context.Context, c Client, query string) (Answers[engine.Term], error) {
	return c.createProlog(ctx, query)
}

// Engine is a pengine.
type Engine struct {
	id        string
	client    Client
	openLimit int  // TODO: use this
	destroy   bool // automatically destroy if true (default)
	dead      bool
}

// ID return this pengine's ID.
func (e *Engine) ID() string {
	return e.id
}

// Ask queries the pengine, returning an answers iterator.
func (e *Engine) Ask(ctx context.Context, query string) (Answers[Solution], error) {
	if e.dead {
		return nil, ErrDead
	}
	opts := e.client.options("prolog")
	opts.Destroy = e.destroy
	query = "ask((" + query + "), " + opts.String() + ")"
	answer, err := e.send(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("pengine ask error: %w", err)
	}
	return newIterator[Solution](e, answer)
}

func (e *Engine) handle(a answer) error {
	switch a.Event {
	case "create":
		e.id = a.ID
		e.openLimit = a.OpenLimit
	case "destroy", "died":
		e.die()
	}
	if a.Answer != nil {
		return e.handle(*a.Answer)
	}
	return nil
}

// Pings this pengine, returning ErrDead if it is dead.
func (e *Engine) Ping(ctx context.Context) error {
	if e.dead {
		return ErrDead
	}

	a, err := e.get(ctx, "ping", "json")
	if err != nil {
		return err
	}
	if err := e.handle(a); err != nil {
		return err
	}
	if e.dead {
		return ErrDead
	}
	return nil
}

// Close destroys this engine. It is usually not necessary to do this as pengines will destroy themselves automatically unless configured differently.
func (e *Engine) Close() error {
	if e.dead {
		return nil
	}
	a, err := e.send(context.Background(), "destroy")
	if err != nil {
		return err
	}
	return e.handle(a)
}

func (e *Engine) die() {
	e.dead = true
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
