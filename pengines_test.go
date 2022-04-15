package pengine

import (
	"reflect"
	"testing"

	"github.com/ichiban/prolog/engine"
)

func TestPengines(t *testing.T) {
	srv := Server{
		URL:   "http://localhost:4242",
		Chunk: 5,
	}

	eng, err := srv.Create()
	if err != nil {
		t.Fatal(err)
	}

	as, err := eng.Ask("member(X, [1, 2.1, a, b(c), [d], should_stop_before_this])")
	if err != nil {
		t.Fatal(err)
	}

	expect := []engine.Term{
		engine.Integer(1),
		engine.Float(2.1),
		engine.Atom("a"),
		&engine.Compound{Functor: "b", Args: []engine.Term{engine.Atom("c")}},
		engine.List(engine.Atom("d")),
	}

	i := 0
	for i < len(expect) && as.Next() {
		want := expect[i]
		got := as.Current()["X"].Prolog()
		if !reflect.DeepEqual(want, got) {
			t.Error("unexpected answer. want:", want, "got:", got)
		}
		i++
	}
	if err := as.Error(); err != nil {
		t.Fatal(err)
	}
	if i != len(expect) {
		t.Error("answer len mismatch. want:", len(expect), "got:", i)
	}

	t.Logf("cumulative time: %v", as.Cumulative())

	// test stop
	if err := as.Close(); err != nil {
		t.Error(err)
	}

	_, err = eng.Ask("true")
	if err != ErrDead {
		t.Error("want:", ErrDead, "got:", err)
	}

	t.Run("server direct query", func(t *testing.T) {
		as, err := srv.Ask("between(1,6,X).")
		if err != nil {
			t.Fatal(err)
		}
		var got []engine.Term
		for as.Next() {
			cur := as.Current()
			got = append(got, cur["X"].Prolog())
		}
		want := []engine.Term{
			engine.Integer(1),
			engine.Integer(2),
			engine.Integer(3),
			engine.Integer(4),
			engine.Integer(5),
			engine.Integer(6),
		}
		if !reflect.DeepEqual(want, got) {
			t.Error("bad results. want:", want, "got:", got)
		}
	})

	t.Run("engine destroy", func(t *testing.T) {
		eng, err := srv.Create()
		if err != nil {
			t.Fatal(err)
		}
		if err := eng.Close(); err != nil {
			t.Fatal(err)
		}
		_, err = eng.Ask("true")
		if err != ErrDead {
			t.Error("want:", ErrDead, "got:", err)
		}
	})
}
