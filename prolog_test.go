package pengine

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/ichiban/prolog"
)

func TestProlog(t *testing.T) {
	eng, err := Client{
		URL: *penginesServerURL,
	}.Create(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	as, err := eng.AskProlog(ctx, "member(X, [1, 1, Y])")
	if err != nil {
		panic(err)
	}

	for as.Next(ctx) {
		t.Logf("answer: %+v", as.Current())
	}
	if err := as.Err(); err != nil {
		t.Error(err)
	}
}

func TestRPC(t *testing.T) {
	p := prolog.New(nil, os.Stdout)
	p.Register3("pengine_rpc", RPC)

	sols, err := p.Query("pengine_rpc('?', between(1,3,X), [chunk(2)]), OK = true.", *penginesServerURL)
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
}
