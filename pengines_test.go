package pengine

import (
	"context"
	"flag"
	"reflect"
	"testing"

	"github.com/ichiban/prolog/engine"
)

var penginesServerURL = flag.String("pengines-server", "http://localhost:4242/pengine", "pengines URL for testing")

func TestPengines(t *testing.T) {
	client := Client{
		URL:   *penginesServerURL,
		Chunk: 5,
		Debug: true,
	}

	ctx := context.Background()

	eng, err := client.Create(ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := eng.Ping(ctx); err != nil {
		t.Error(err)
	}

	as, err := eng.Ask(ctx, "member(X, [1, 2.1, 'あ', b(c), [d], should_stop_before_this])")
	if err != nil {
		t.Fatal(err)
	}

	expect := []engine.Term{
		engine.Integer(1),
		engine.Float(2.1),
		engine.Atom("あ"),
		engine.Atom("b").Apply(engine.Atom("c")),
		engine.List(engine.Atom("d")),
	}

	i := 0
	for i < len(expect) && as.Next(ctx) {
		want := expect[i]
		got := as.Current()["X"].Prolog()
		if !reflect.DeepEqual(want, got) {
			t.Error("unexpected answer. want:", want, "got:", got)
		}
		i++
	}
	if err := as.Err(); err != nil {
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

	_, err = eng.Ask(ctx, "true")
	if err != ErrDead {
		t.Error("want:", ErrDead, "got:", err)
	}

	t.Run("server direct query", func(t *testing.T) {
		as, err := client.Ask(ctx, "between(1,6,X).")
		if err != nil {
			t.Fatal(err)
		}
		var got []engine.Term
		for as.Next(ctx) {
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
		eng, err := client.Create(ctx, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := eng.Close(); err != nil {
			t.Fatal(err)
		}
		_, err = eng.Ask(ctx, "true")
		if err != ErrDead {
			t.Error("want:", ErrDead, "got:", err)
		}
	})
}
