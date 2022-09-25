package pengine

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/ichiban/prolog/engine"
)

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
		iterator: iterator[engine.Term]{
			eng: eng,
		},
	}
	return p
}

type prologAnswers struct {
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
		debug:   c.Debug,
	}
	as := newProlog(eng)
	opts := c.options("prolog")
	opts.Destroy = true
	opts.Ask = query
	opts.Template = query

	evt, err := eng.postProlog("create", opts)
	if err != nil {
		return nil, fmt.Errorf("pengine create error: %w", err)
	}
	return as, as.handle(ctx, evt)
}

func (p *prologAnswers) handle(ctx context.Context, a string) error {
	interpreter := defaultInterpreter
	if p.eng.client.Interpreter != nil {
		interpreter = p.eng.client.Interpreter
	}
	parser := interpreter.Parser(strings.NewReader(a), nil)
	t, err := parser.Term()
	if err != nil {
		return fmt.Errorf("pengines: failed to parse response: %w", err)
	}

	event, ok := t.(engine.Compound)
	if !ok {
		return fmt.Errorf("unexpected event type: %T (value: %v)", t, t)
	}
	return p.handleEvent(event)
}

func (p *prologAnswers) handleEvent(t engine.Compound) error {
	/*
		% Original script looked like:

		success(ID, Terms, Projection, Time, More) :-
			'$pengine_success'(ID, Terms, Projection, Time, More).

		failure(ID, Time) :-
			'$pengine_failure'(ID, Time).

		error(ID, Term) :-
			'$pengine_error'(ID, Term).

		create(ID, Data) :-
			( member(slave_limit(Limit), Data) -> true
			; Limit = 0
			),
			!,
			'$pengine_create'(ID, Limit),
			( member(answer(Goal), Data) -> call(Goal)
			; true
			),
			!.

		destroy(ID, Result) :-
			call(Result),
			'$pengine_destroy'(ID).

		output(ID, Term) :-
			'$pengine_output'(ID, Term).

	*/
	switch t.Functor() {
	case "success": // success/5
		// id, results, projection, time, more
		return p.onSuccess(t.Arg(0), t.Arg(1), t.Arg(2), t.Arg(3), t.Arg(4))
	case "failure": // failure/2
		// id, time
		return p.onFailure(t.Arg(0), t.Arg(1))
	case "error": // error/2
		// id, ball
		return p.onError(t.Arg(0), t.Arg(1))
	case "create": // create/2
		// id, list
		return p.onCreate(t.Arg(0), t.Arg(1))
	case "destroy": // destroy/2
		// id, event
		return p.onDestroy(t.Arg(0), t.Arg(1))
	case "output": // output/2
		// TODO: unimplemented
		return p.onOutput(t.Arg(0), t.Arg(1))
	case "prompt": // prompt/2
		// TODO
	}
	return nil
}

func (p *prologAnswers) accumulate(time engine.Term) bool {
	n, ok := time.(engine.Float)
	p.cum += float64(n)
	return ok
}

func (p *prologAnswers) onSuccess(id, results, projection, time, more engine.Term) error {
	iter := engine.ListIterator{List: results, Env: nil}
	for iter.Next() {
		cur := resolve(iter.Current(), nil, nil)
		p.buf = append(p.buf, cur)
		p.good++
	}
	if err := iter.Err(); err != nil {
		return err
	}

	p.accumulate(time)

	m, ok := more.(engine.Atom)
	if !ok {
		return engine.TypeError(engine.ValidTypeAtom, more, nil)
	}
	p.more = m == "true"

	return nil
}

func (p *prologAnswers) onFailure(id, time engine.Term) error {
	p.bad++
	p.accumulate(time)
	return nil
}

func (p *prologAnswers) onError(id, ball engine.Term) error {
	p.err = engine.NewException(ball, nil)
	return nil
}

func (p *prologAnswers) onOutput(id, term engine.Term) error {
	// TODO(guregu): currently unimplemented.
	return nil
}

func (p *prologAnswers) onDestroy(id, t engine.Term) error {
	p.eng.die()
	goal, ok := t.(engine.Compound)
	if ok {
		return p.handleEvent(goal)
	}
	return nil
}

func (p *prologAnswers) onCreate(id, data engine.Term) error {
	atomID, ok := id.(engine.Atom)
	if !ok {
		return fmt.Errorf("expected atom ID: got %T (value: %v)", id, id)
	}
	p.eng.id = string(atomID)

	iter := engine.ListIterator{List: data}
	for iter.Next() {
		cur := iter.Current()
		switch t := cur.(type) {
		case engine.Compound:
			switch t.Functor() {
			case "slave_limit":
				n, ok := t.Arg(0).(engine.Integer)
				if ok {
					p.eng.openLimit = int(n)
				}
			case "answer":
				goal, ok := t.Arg(0).(engine.Compound)
				if ok {
					defer p.handleEvent(goal)
				}
			}
		}
	}
	return iter.Err()
}

// resolve is a version of Simplify that attempts to loosely occurs-check itself to prevent infinite loops
func resolve(t engine.Term, env *engine.Env, seen map[engine.Term]struct{}) engine.Term {
	return env.Resolve(t)
}
