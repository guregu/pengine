package pengine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ichiban/prolog/engine"
)

// Solution is a mapping of variable names to values.
//
//	answers, err := Ask[Solution](ctx, "between(1,6,X)")
//	// ...
//	for as.Next() {
//		cur := as.Current()
//		// grab variables by name
//		x := cur["X"]
//	}
type Solution map[string]Term

// Term represents a Prolog term.
// One of the fields should be "truthy".
// This can be handy for parsing query results in JSON format.
type Term struct {
	Atom       *string
	Number     *json.Number
	Compound   *Compound
	Variable   *string
	Boolean    *bool
	List       []Term
	Dictionary map[string]Term
	Null       bool
}

// UnmarshalJSON implements json.Unmarshaler.
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
			variable := x
			t.Variable = &variable
		} else {
			atom := x
			t.Atom = &atom
		}
	case bool:
		boolean := x
		t.Boolean = &boolean
	case []any:
		return json.Unmarshal(b, &t.List)
	case map[string]any:
		if _, ok := x["functor"]; ok {
			return json.Unmarshal(b, &t.Compound)
		}
		rawDict := make(map[string]json.RawMessage, len(x))
		if err := json.Unmarshal(b, &rawDict); err != nil {
			return err
		}
		t.Dictionary = make(map[string]Term, len(rawDict))
		for k, raw := range rawDict {
			var v Term
			if err := json.Unmarshal(raw, &v); err != nil {
				return err
			}
			t.Dictionary[k] = v
		}
	case nil:
		t.Null = true
	default:
		panic(fmt.Errorf("pengine: can't parse term of type %T", x))
	}

	return nil
}

// Prolog converts this term to an ichiban/prolog term.
// Because pengine's JSON format is lossy in terms of Prolog types, this might not always be accurate.
// There is ambiguity between atoms, strings, and variables.
// If you are mainly dealing with Prolog terms, use AskProlog to use the Prolog format instead.
func (t Term) Prolog() engine.Term {
	switch {
	case t.Atom != nil:
		return engine.Atom(*t.Atom)
	case t.Number != nil:
		// TODO(guregu): fix/document/make optional the Int/Float detection.
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
		args := make([]engine.Term, 0, len(t.Compound.Args))
		for _, arg := range t.Compound.Args {
			args = append(args, arg.Prolog())
		}
		return engine.Atom(t.Compound.Functor).Apply(args...)
	case t.Variable != nil:
		// TODO(guregu): what should this be? engine.NewVariable? Is this even useful?
		return engine.Variable(*t.Variable)
	case t.Boolean != nil:
		// TODO(guregu): use `@(true)` instead?
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
		return engine.Atom("null") // TODO(guregu): use `@(null)`?
	}
	return nil
}

// Compound is a Prolog compound: functor(args0, args1, ...).
type Compound struct {
	Functor string `json:"functor"`
	Args    []Term `json:"args"`
}

func escapeAtom(atom string) string {
	return stringify(engine.Atom(atom))
}
