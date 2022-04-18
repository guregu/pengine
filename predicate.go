package pengine

import (
	"context"
	"errors"
	"strings"

	"github.com/ichiban/prolog"
	"github.com/ichiban/prolog/engine"
)

// defaultInterpreter is kept around to keep a reference to the default operator table. Hacky.
var defaultInterpreter = prolog.New(nil, nil)

// RPC is like pengine_rpc/3 from SWI, provided for as a native predicate for ichiban/prolog.
//
// Supports the following options: application(Atom), chunk(Integer), src_url(Atom).
//
// See: https://www.swi-prolog.org/pldoc/man?predicate=pengine_rpc/3
func RPC(url, query, options engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	client := new(Client)
	switch url := env.Resolve(url).(type) {
	case engine.Atom:
		client.URL = string(url)
	case engine.Variable:
		return engine.Error(engine.ErrInstantiation)
	default:
		return engine.Error(engine.TypeErrorAtom(url))
	}

	query = env.Simplify(query)
	var q strings.Builder
	if err := engine.Write(&q, query, env, engine.WithQuoted(true), defaultInterpreter.WithIgnoreOps(false)); err != nil {
		return engine.Error(err)
	}

	iter := engine.ListIterator{List: options, Env: env}
	for iter.Next() {
		cur := env.Resolve(iter.Current())
		switch x := cur.(type) {
		case *engine.Compound:
			switch x.Functor {
			case "application":
				a, ok := env.Resolve(x.Args[0]).(engine.Atom)
				if !ok {
					return engine.Error(engine.TypeErrorAtom(x.Args[0]))
				}
				client.Application = string(a)
			case "chunk":
				n, ok := env.Resolve(x.Args[0]).(engine.Integer)
				if !ok {
					return engine.Error(engine.TypeErrorInteger(x.Args[0]))
				}
				client.Chunk = int(n)
			case "src_url":
				a, ok := env.Resolve(x.Args[0]).(engine.Atom)
				if !ok {
					return engine.Error(engine.TypeErrorAtom(x.Args[0]))
				}
				client.SourceURL = string(a)
			}
		}
	}
	if err := iter.Err(); err != nil {
		return engine.Error(err)
	}

	as, err := client.createProlog(context.Background(), q.String())
	if err != nil {
		return engine.Error(err)
	}

	return engine.Delay(func(context.Context) *engine.Promise {
		return engine.Repeat(func(ctx context.Context) *engine.Promise {
			if as.Next(ctx) {
				cur := as.Current()
				return engine.Unify(query, cur, k, env)
			}
			as.Close()
			switch {
			case errors.Is(as.Err(), ErrFailed):
				return engine.Bool(true)
			case as.Err() != nil:
				return engine.Error(as.Err())
			}
			return engine.Bool(true)
		})
	})
}
