package pengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ichiban/prolog/engine"
)

type Solution map[string]Term

type Term struct {
	Atom     *Atom
	Number   *Number
	Compound *Compound
	Variable *Variable
	Boolean  *Bool
	List     []Term
	Null     bool
}

func (t *Term) UnmarshalJSON(b []byte) error {
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return err
	}

	switch x := v.(type) {
	case json.Number:
		t.Number = &x
	case string:
		// TODO: need to figure out how to disambiguate var(_) / atom('_')
		// maybe you can't? or need to use prolog instead of json format?
		if x == "_" {
			variable := Variable(x)
			t.Variable = &variable
		} else {
			atom := Atom(x)
			t.Atom = &atom
		}
	case bool:
		boolean := Bool(x)
		t.Boolean = &boolean
	case []any:
		return json.Unmarshal(b, &t.List)
	case map[string]any:
		return json.Unmarshal(b, &t.Compound)
	case nil:
		t.Null = true
	default:
		panic(fmt.Errorf("pengine: can't parse term of type %T", x))
	}

	return nil
}

func (t Term) Prolog() engine.Term {
	switch {
	case t.Atom != nil:
		return engine.Atom(*t.Atom)
	case t.Number != nil:
		// TODO: fix this
		nstr := string(*t.Number)
		if strings.ContainsRune(nstr, '.') {
			f, err := strconv.ParseFloat(nstr, 64)
			if err != nil {
				panic(err)
			}
			return engine.Float(f)
		}
		n, err := strconv.ParseInt(nstr, 10, 64)
		if err != nil {
			panic(err)
		}
		return engine.Integer(n)
	case t.Compound != nil:
		c := &engine.Compound{
			Functor: engine.Atom(t.Compound.Functor),
			Args:    make([]engine.Term, 0, len(t.Compound.Args)),
		}
		for _, arg := range t.Compound.Args {
			c.Args = append(c.Args, arg.Prolog())
		}
		return c
	case t.Variable != nil:
		return engine.NewVariable() // TODO: probably not necessary
	case t.Boolean != nil:
		// TODO: use @(true) instead?
		if *t.Boolean {
			return engine.Atom("true")
		} else {
			return engine.Atom("false")
		}
	case t.List != nil:
		list := make([]engine.Term, 0, len(t.List))
		for _, member := range t.List {
			list = append(list, member.Prolog())
		}
		return engine.List(list...)
	case t.Null:
		return engine.Atom("null") // TODO: use @(null)? or ''?
	}
	return nil
}

type Atom string

type Number = json.Number

type Compound struct {
	Functor string `json:"functor"`
	Args    []Term `json:"args"`
}

type Variable string

type Bool bool
