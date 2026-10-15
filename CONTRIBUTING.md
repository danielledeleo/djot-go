# Contributing

Thanks for your interest in `djot-go`! Here's how to get started.

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

The djot.js test suite lives in `testdata/official` and is run by
`TestOfficial` in `official_test.go`. If you fix a parsing bug, check whether
any of these tests change status.

To update the suite, copy `test/*.test` from djot.js, record the commit in
`testdata/README`, and set the same commit as `DJOT_JS_COMMIT` in
`testdata/difftest/Dockerfile`.

### Differential tests

`diff_test.go` compares `djot-go` output with djot.js, built from GitHub at
the commit the test suite was taken from. It runs inside Docker:

```
docker build -f testdata/difftest/Dockerfile -t djot-diff .
docker run --rm djot-diff
```

The build context is the repository root, not the difftest directory: the
Dockerfile copies `go.mod` and the package sources from the root.

Most cases are machine-generated punctuation soup used to probe edge cases, and
nine of the 279 diverge from djot.js. They are not part of `go test ./...` or
CI; the test sits behind a `difftest` build tag. The djot.js test suite in
`testdata/official` is the compatibility gate, and it runs by default. The
divergences are listed in [docs/compatibility.md](docs/compatibility.md).

## Making changes

1. Keep changes focused — one bug fix or feature per PR.
2. Add or update tests for any behavioral change.
3. Run `go test ./...` and `go vet ./...` before submitting.
4. Match the existing code style (gofmt, no unnecessary abstractions).

## Reporting bugs

Open an issue with:

- The Djot input that triggers the bug
- Expected vs actual HTML output
- If possible, what djot.js produces for the same input

## License

By contributing, you agree that your contributions will be licensed under the
MIT License.
