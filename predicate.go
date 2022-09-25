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

// RPCWithInterpreter is like RPC but allows you to specify an interpreter to use for parsing results.
// Useful for handling custom operators.
func RPCWithInterpreter(p *prolog.Interpreter) func(url, query, options engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	return rpc(p)
}

// RPC is like pengine_rpc/3 from SWI, provided for as a native predicate for ichiban/prolog.
// This is a native predicate for Prolog. To use the API from Go, use AskProlog.
//
// Supports the following options: application(Atom), chunk(Integer), src_text(Atom), src_url(Atom), debug(Boolean).
//
// See: https://www.swi-prolog.org/pldoc/man?predicate=pengine_rpc/3
func RPC(url, query, options engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	return defaultRPC(url, query, options, k, env)
}

var defaultRPC = rpc(nil)

func rpc(p *prolog.Interpreter) func(url, query, options engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	return func(url, query, options engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
		client := &Client{
			Interpreter: p,
		}
		switch url := env.Resolve(url).(type) {
		case engine.Atom:
			client.URL = string(url)
		case engine.Variable:
			return engine.Error(engine.InstantiationError(env))
		default:
			return engine.Error(engine.TypeError(engine.ValidTypeAtom, url, env))
		}

		query = env.Simplify(query)
		var q strings.Builder
		if err := engine.WriteTerm(&q, query, &engine.WriteOptions{
			Quoted:    true,
			IgnoreOps: true,
		}, env); err != nil {
			return engine.Error(err)
		}

		iter := engine.ListIterator{List: options, Env: env}
		for iter.Next() {
			cur := env.Resolve(iter.Current())
			switch x := cur.(type) {
			case engine.Compound:
				switch x.Functor() {
				case "application":
					str := term2str(x.Arg(0), env)
					if str == "" {
						return engine.Error(engine.TypeError(engine.ValidTypeAtom, x.Arg(0), env))
					}
					client.Application = str
				case "chunk":
					n, ok := env.Resolve(x.Arg(0)).(engine.Integer)
					if !ok {
						return engine.Error(engine.TypeError(engine.ValidTypeAtom, x.Arg(0), env))
					}
					client.Chunk = int(n)
				case "src_text":
					str := term2str(x.Arg(0), env)
					if str == "" {
						return engine.Error(engine.TypeError(engine.ValidTypeAtom, x.Arg(0), env))
					}
					client.SourceText = str
				case "src_url":
					str := term2str(x.Arg(0), env)
					if str == "" {
						return engine.Error(engine.TypeError(engine.ValidTypeAtom, x.Arg(0), env))
					}
					client.SourceURL = str
				case "debug":
					str := term2str(x.Arg(0), env)
					if str == "" {
						return engine.Error(engine.TypeError(engine.ValidTypeAtom, x.Arg(0), env))
					}
					client.Debug = str == "true"
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

		return doRPC(as, query, k, env)
	}
}

func doRPC(as *prologAnswers, query engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	var done bool
	return engine.Delay(func(ctx context.Context) *engine.Promise {
		if as.Next(ctx) {
			cur := as.Current()
			return engine.Unify(query, cur, k, env)
		}
		done = true
		if err := as.Err(); err != nil && !errors.Is(err, ErrFailed) {
			return engine.Error(err)
		}
		return engine.Bool(false)
	}, func(ctx context.Context) *engine.Promise {
		if !done {
			return doRPC(as, query, k, env)
		}
		return engine.Bool(false)
	})
}

func stringify(t engine.Term) string {
	var q strings.Builder
	_ = engine.WriteTerm(&q, t, &engine.WriteOptions{
		Quoted:    true,
		IgnoreOps: false,
	}, nil)
	return q.String()
}

func term2str(t engine.Term, env *engine.Env) string {
	switch t := env.Resolve(t).(type) {
	case engine.Atom:
		return string(t)
	}
	return ""
}
