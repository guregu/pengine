# pengine [![GoDoc](https://godoc.org/github.com/guregu/pengine?status.svg)](https://godoc.org/github.com/guregu/pengine)
`import "github.com/guregu/pengine"`

[Pengines](https://www.swi-prolog.org/pldoc/doc_for?object=section(%27packages/pengines.html%27)), Prolog Engines, client for Go.
Pengines's motto is "Web Logic Programming Made Easy". This library lets you query SWI-Prolog from Go and from [ichiban/prolog](https://github.com/ichiban/prolog), a Prolog interpreter written in Go.

**Development Status**: working towards beta. Feedback and contributions welcome!

## Usage

This library supports both the JSON and Prolog format APIs. Additionally, an RPC predicate for ichiban/prolog is provided.

### Configuration

```go
client := pengine.Client{
    // URL of the pengines server, required.
    URL: "http://localhost:4242/pengine",

    // Application name, optional.
    Application: "pengines_sandbox",
    // Chunk is the number of query results to accumulate in one response. 1 by default.
    Chunk: 10,
    // SourceText is Prolog source code to load (optional, currently only supported for the JSON format).
    SourceText: "awesome(prolog).\n",
    // SourceURL specifies a URL of Prolog source for the pengine to load (optional).
    SourceURL: "https://example.com/script.pl",
}
```

### JSON API

`client.Ask` returns an `Answers[Solutions]` iterator, but you can use the generic `pengine.Ask[T]` function to use your own types.

```go
answers, err := client.Ask(ctx, "between(1,6,X)")
if err != nil {
	panic(err)
}
var got []json.Number
for as.Next() {
	cur := as.Current()
	x := cur["X"]
	got = append(got, x.Number)
}
// got = {"1", "2", "3", ...}
if err := answers.Error(); err != nil {
	panic(err)
}
```

You can also use `client.Create` to create a pengine and `Ask` it later. If you need to stop a query early or destroy a pengine whose automatic destruction was disabled, you can call `client.Close`.

### Prolog API

`client.AskProlog` returns `ichiban/prolog/engine.Term` objects. This uses an ichiban/prolog interpreter to handle results.
You can also call `pengine.Term.Prolog()` to get Prolog terms from the JSON results, but they might be lossy in terms of Prolog typing.

### RPC for ichiban/prolog

You can call remote pengines from [ichiban/prolog](https://github.com/ichiban/prolog), a Go Prolog, as well.

[`pengine_rpc/3`](https://www.swi-prolog.org/pldoc/man?predicate=pengine_rpc/3) mostly works like its SWI-Prolog counterpart.
Not all the options are implemented yet, and it is very untested.

```go
interpreter.Register3("pengine_rpc", pengine.RPC)
```

```prolog
:- pengine_rpc('http://localhost:4242/pengine', between(1,50,X), [chunk(10)]), write(X), nl.
% 1 2 3 4 ...
```

## Tests

Currently the tests are rather manual:

```prolog
% from swipl
consult(example_server).
```

```bash
# from OS terminal
go test -v
```

Change the pengines server URL used by the tests with the `--pengines-server` command line flag.

## Other languages

Check out these pengines clients for other languages.

- **[Programming against SWISH](https://github.com/SWI-Prolog/swish/tree/master/client)** is an up-to-date list of clients.
- Erlang: [erl_pengine](https://github.com/Limmen/erl_pengine)
- Java: [JavaPengine](https://github.com/Anniepoo/JavaPengine)
- Javascript: [pengines.js](https://pengines.swi-prolog.org/docs/documentation.html)
- Python: [PythonPengines](https://github.com/ian-andrich/PythonPengines)
- Ruby: [RubyPengines](https://github.com/simularity/RubyPengine)

## Thanks

- Torbjörn Lager, Jan Wielemaker, and everyone else who has contributed to SWI-Prolog and Pengines.
- @ian-andrich for the Python implementation, which was a handy reference for making this.
- @ichiban for the awesome Prolog interpreter in Go.
- @triska for the wonderful tutorial series [The Power of Prolog](https://www.metalevel.at/prolog), which started me on my logic programming journey.

## License

BSD 2-clause.
