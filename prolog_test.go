package pengine

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/ichiban/prolog"
	"github.com/ichiban/prolog/engine"
)

func TestProlog(t *testing.T) {
	eng, err := Client{
		URL:        *penginesServerURL,
		SourceText: "'子'(X, List) :- member(X, List).\n",
		Debug:      true,
	}.Create(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	as, err := eng.AskProlog(ctx, "'子'(X, ['あ', 1, Y])")
	if err != nil {
		t.Fatal(err)
	}

	for as.Next(ctx) {
		t.Logf("answer: %+v", as.Current())
		cmp, ok := as.Current().(*engine.Compound)
		if !ok {
			t.Fatal("not a compound", as.Current())
		}
		if cmp.Functor != "子" {
			t.Error("unexpected functor. want: 子 got:", cmp.Functor)
		}
	}
	if err := as.Err(); err != nil {
		t.Error(err)
	}
}

func TestRPC(t *testing.T) {
	p := prolog.New(nil, os.Stdout)
	p.Register3("pengine_rpc", RPC)

	sols, err := p.Query("pengine_rpc('?', between(1,3,X), [chunk(2), debug(true)]), OK = true.", *penginesServerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer sols.Close()

	n := 0
	var got []int
	for sols.Next() {
		var solution struct {
			X  int
			OK string
		}
		if err := sols.Scan(&solution); err != nil {
			panic(err)
		}
		t.Log("solution:", solution)
		got = append(got, solution.X)
		if solution.OK != "true" {
			t.Error("not ok:", solution)
		}
		n++
	}

	if err := sols.Err(); err != nil {
		t.Error(err)
	}

	want := []int{1, 2, 3}
	if !reflect.DeepEqual(want, got) {
		t.Error("bad rpc results. want:", want, "got:", got)
	}

	t.Run("complex", func(t *testing.T) {
		// sol := p.QuerySolution(`pengine_rpc(?, (A=a+b->true ; false), [debug(true)]).`, *penginesServerURL)
		sol := p.QuerySolution("pengine_rpc('?', (Foo = bar+baz, (Foo = A+B -> true ; A = x)), [debug(true)]).", *penginesServerURL)
		if err := sol.Err(); err != nil {
			t.Fatal(err)
		}
		type result struct {
			A string
			B string
		}
		want := result{A: "bar", B: "baz"}
		var got result
		if err := sol.Scan(&got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Error("bad rpc results. want:", want, "got:", got)
		}
	})

	t.Run("fail", func(t *testing.T) {
		sols := p.QuerySolution("pengine_rpc('?', fail, []), OK = false.", *penginesServerURL)
		var val struct {
			OK string
		}
		if err := sols.Scan(&val); err != prolog.ErrNoSolutions {
			t.Fatal("wanted:", prolog.ErrNoSolutions, "got:", err)
		}
		if val.OK == "false" {
			t.Error("expected empty, got:", val.OK)
		}
	})

	t.Run("throw", func(t *testing.T) {
		sols := p.QuerySolution("catch(pengine_rpc('?', throw(hello(world)), [debug(true)]), hello(Planet), (Caught = Planet)).", *penginesServerURL)
		var val struct {
			Caught string
		}
		if err := sols.Scan(&val); err != nil {
			t.Fatal(err)
		}
		if val.Caught != "world" {
			t.Error("expected world, got:", val.Caught)
		}
	})
}
