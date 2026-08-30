# Contributing

Thanks for your interest in djot-go! Here's how to get started.

## Development

```
git clone https://github.com/danielledeleo/djot-go
cd djot-go
go test ./...
```

## Tests

Run the full suite:

```
go test ./...
```

Run benchmarks:

```
go test -bench=. -benchmem ./...
```

Run the fuzzer:

```
go test -fuzz=FuzzParse -fuzztime=30s ./...
```

### Spec tests

The official djot spec tests live in `testdata/` and are run by
`TestOfficial` in `official_test.go`. If you fix a parsing bug, check whether
any spec tests change status.

### Differential tests

`diff_test.go` compares djot-go output against the djot.js reference
implementation inside Docker. To run:

```
docker build -f testdata/difftest/Dockerfile -t djot-diff .
docker run --rm djot-diff
```

The build context is the repository root, not the difftest directory: the
Dockerfile copies `go.mod` and the package sources from the root.

These cases are machine-generated punctuation soup used to probe edge cases, and
ten of the 279 still diverge from djot.js. They are not part of `go test ./...`
or CI -- the test sits behind a `difftest` build tag. The official conformance
fixtures in `testdata/official` are the compatibility gate, and those run by
default. The surviving divergences are catalogued under "Known divergences" in
[docs/compatibility.md](docs/compatibility.md).

## Making changes

1. Keep changes focused — one bug fix or feature per PR.
2. Add or update tests for any behavioral change.
3. Run `go test ./...` and `go vet ./...` before submitting.
4. Match the existing code style (gofmt, no unnecessary abstractions).

## Reporting bugs

Open an issue with:

- The djot input that triggers the bug
- Expected vs actual HTML output
- If possible, what djot.js produces for the same input

## License

By contributing, you agree that your contributions will be licensed under the
MIT License.
