package pengine

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/ichiban/prolog"
	"github.com/ichiban/prolog/engine"
)

//go:embed receive.pl
var receiveScript string

// AskProlog queries this engine with Prolog format results.
// Queries must be in Prolog syntax without the terminating period or linebreak, such as:
//
//	between(1,3,X)
//
// Because ichiban/prolog is used to interpret results, using SWI's nonstandard syntax extensions like dictionaries may break it.
func (e *Engine) AskProlog(ctx context.Context, query string) (Answers[engine.Term], error) {
	as := newProlog(e)
	opts := e.client.options("prolog")
	query = "ask((" + query + "), " + opts.String() + ")"
	a, err := e.sendProlog(ctx, query)
	if err != nil {
		return nil, err
	}
	return as, as.handle(ctx, a)
}

func newProlog(eng *Engine) *prologAnswers {
	p := &prologAnswers{
		Interpreter: newInterpreter(),
		eng:         eng,
	}
	p.Register5("$pengine_success", p.onSuccess)
	p.Register2("$pengine_failure", p.onFailure)
	p.Register2("$pengine_error", p.onError)
	p.Register2("$pengine_output", p.onOutput)
	p.Register1("$pengine_destroy", p.onDestroy)
	p.Register2("$pengine_create", p.onCreate)

	if err := p.Exec(receiveScript); err != nil {
		panic(err)
	}
	return p
}

type prologAnswers struct {
	*prolog.Interpreter
	eng *Engine
	iterator[engine.Term]
}

func (as *prologAnswers) Next(ctx context.Context) bool {
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
		a, err := as.eng.sendProlog(ctx, "next")
		if err != nil {
			as.err = err
			return false
		}
		if err := as.handle(ctx, a); err != nil {
			as.err = err
			return false
		}
		goto more
	}
	return false
}

func (c Client) createProlog(ctx context.Context, query string) (*prologAnswers, error) {
	if c.URL == "" {
		return nil, fmt.Errorf("pengine: Server URL not set")
	}

	eng := &Engine{
		client:  c,
		destroy: true,
	}
	as := newProlog(eng)
	opts := c.options("prolog")
	opts.Ask = query
	opts.Template = query

	evt, err := eng.postProlog("create", opts)
	if err != nil {
		return nil, fmt.Errorf("pengine create error: %w", err)
	}
	return as, as.handle(ctx, evt)
}

func (p *prologAnswers) handle(ctx context.Context, a string) error {
	return p.ExecContext(ctx, ":- "+a)
}

func (p *prologAnswers) onSuccess(id, results, projection, time, more engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	iter := engine.ListIterator{List: results, Env: env}
	for iter.Next() {
		cur := resolve(iter.Current(), env, nil)
		p.buf = append(p.buf, cur)
		p.good++
	}
	if err := iter.Err(); err != nil {
		return engine.Error(err)
	}

	n, ok := env.Resolve(time).(engine.Float)
	if !ok {
		return engine.Error(engine.TypeErrorFloat(time))
	}
	p.cum += float64(n)

	m, ok := env.Resolve(more).(engine.Atom)
	if !ok {
		return engine.Error(engine.TypeErrorAtom(more))
	}
	p.more = m == "true"

	return k(env)
}

func (p *prologAnswers) onFailure(id, time engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	p.bad++

	n, ok := env.Resolve(time).(engine.Float)
	if !ok {
		return engine.Error(engine.TypeErrorFloat(time))
	}
	p.cum += float64(n)

	return k(env)
}

func (p *prologAnswers) onError(id, ball engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	p.err = &engine.Exception{Term: env.Simplify(ball)}
	return k(env)
}

func (p *prologAnswers) onOutput(id, term engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	// TODO(guregu): current unimplemented.
	return k(env)
}

func (p *prologAnswers) onDestroy(id engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	p.eng.die()
	return k(env)
}

func (p *prologAnswers) onCreate(id, limit engine.Term, k func(*engine.Env) *engine.Promise, env *engine.Env) *engine.Promise {
	p.eng.id = string(env.Resolve(id).(engine.Atom))
	if lim, ok := limit.(engine.Integer); ok {
		p.eng.openLimit = int(lim)
	}
	return k(env)
}

// resolve is a version of Simplify that attempts to loosely occurs-check itself to prevent infinite loops
func resolve(t engine.Term, env *engine.Env, seen map[engine.Term]struct{}) engine.Term {
	if seen == nil {
		seen = make(map[engine.Term]struct{})
	}

	if _, ok := seen[t]; ok {
		return t
	}
	seen[t] = struct{}{}

	switch t := env.Resolve(t).(type) {
	case *engine.Compound:
		for i, arg := range t.Args {
			t.Args[i] = resolve(arg, env, seen)
		}
		return t
	default:
		return t
	}
}

// newInterpreter returns a minimal Prolog interpreter.
func newInterpreter() *prolog.Interpreter {
	i := prolog.Interpreter{}
	i.Register1(`\+`, i.Negation)
	i.Register1("call", i.Call)
	i.Register2("call", i.Call1)
	i.Register3("call", i.Call2)
	i.Register4("call", i.Call3)
	i.Register5("call", i.Call4)
	i.Register6("call", i.Call5)
	i.Register7("call", i.Call6)
	i.Register8("call", i.Call7)
	i.Register2("=", engine.Unify)
	i.Register2("unify_with_occurs_check", engine.UnifyWithOccursCheck)
	i.Register3("op", i.Op)
	i.Register1("built_in", i.BuiltIn)

	err := i.Exec(`		
		:-(op(1200, xfx, :-)).
		:-(op(1200, xfx, -->)).
		:-(op(1200, fx, :-)).
		:-(op(1200, fx, ?-)).
		:-(op(1105, xfy, '|')).
		:-(op(1100, xfy, ;)).
		:-(op(1050, xfy, ->)).
		:-(op(1000, xfy, ',')).
		:-(op(900, fy, \+)).
		:-(op(700, xfx, =)).
		:-(op(700, xfx, \=)).
		:-(op(700, xfx, ==)).
		:-(op(700, xfx, \==)).
		:-(op(700, xfx, @<)).
		:-(op(700, xfx, @=<)).
		:-(op(700, xfx, @>)).
		:-(op(700, xfx, @>=)).
		:-(op(700, xfx, is)).
		:-(op(700, xfx, =:=)).
		:-(op(700, xfx, =\=)).
		:-(op(700, xfx, <)).
		:-(op(700, xfx, =<)).
		:-(op(700, xfx, =\=)).
		:-(op(700, xfx, >)).
		:-(op(700, xfx, >=)).
		:-(op(700, xfx, =..)).
		:-(op(500, yfx, +)).
		:-(op(500, yfx, -)).
		:-(op(500, yfx, /\)).
		:-(op(500, yfx, \/)).
		:-(op(400, yfx, *)).
		:-(op(400, yfx, /)).
		:-(op(400, yfx, //)).
		:-(op(400, yfx, div)).
		:-(op(400, yfx, rem)).
		:-(op(400, yfx, mod)).
		:-(op(400, yfx, <<)).
		:-(op(400, yfx, >>)).
		:-(op(200, xfx, **)).
		:-(op(200, xfy, ^)).
		:-(op(200, fy, \)).
		:-(op(200, fy, +)).
		:-(op(200, fy, -)).
		:-(op(100, xfx, @)).
		:-(op(50, xfx, :)).

		:- built_in(true/0).
		true.

		:- built_in(fail/0).
		fail :- \+true.

		:- built_in(','/2).
		P, Q :- call((P, Q)).

		:- built_in(';'/2).
		If -> Then; _ :- If, !, Then.
		_ -> _; Else :- !, Else.
		P; Q :- call((P; Q)).

		:- built_in('->'/2).
		If -> Then :- If, !, Then.

		:- built_in(!/0).
		! :- !.

		:- built_in(member/2).
		member(X, [X|_]).
		member(X, [_|Xs]) :- member(X, Xs).
	`)
	if err != nil {
		panic(err)
	}
	return &i
}
