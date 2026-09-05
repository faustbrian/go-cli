# CLI benchmark harness

This internal, non-releasable module compares equivalent command construction
and prepared dispatch across `go-cli`, Cobra, `urfave/cli`, Kong, and the
standard `flag` package. It also preserves differential parser evidence against
the former Cobra-backed implementation.

The module exists only for Golib engineering verification. It has no supported
installation path, public package, semantic-version release, or runtime
dependency relationship with consumers. Applications should depend on the
public [`go-cli` module](../README.md), not this harness.

The harness requires Go 1.26.6 or newer.

## Run

From the repository root, use the repository workspace to compare the current
root source:

```console
GOWORK="$PWD/go.work" go test ./benchmarks/... \
  -run '^TestOwnedParserMatchesTheFormerCobraAdapter$' -count=1
GOWORK="$PWD/go.work" go test ./benchmarks/... -run '^$' \
  -bench '^BenchmarkEquivalent(Construction|Dispatch)$' \
  -benchmem -benchtime=100ms -count=10
```

The root module's benchmark gate runs the same comparison through the
checked-in workspace. Use the exact supported Go toolchain, keep the machine
idle, retain the raw samples, and record operating-system, architecture, CPU,
commit, sample count, and benchmark duration before publishing comparative
evidence. Analyze multiple samples statistically; do not publish only the
fastest run.

## Comparison boundary

Construction measures the setup performed by each library for the fixture.
Dispatch reuses a prepared command graph, parses `deploy --force target`,
validates equivalent inputs, encodes the same result as JSON, and writes it to
`io.Discard`. The standard `flag` case is a parsing floor without a command
graph. Differences in supported features, diagnostics, completion, lifecycle,
concurrency guarantees, and application integration remain outside this
narrow workload.

The differential parser test compares the owned parser with the former Cobra
adapter across options, aliases, nesting, negative values, help, version, and
failure categories. It is compatibility evidence, not a performance result.

Do not use this harness as a universal framework ranking. Measure the actual
command shape, output mode, lifecycle, concurrency, and correctness
requirements of the application being built.

## Navigation

- [Benchmark source](compare_test.go)
- [Parser differential evidence](parser_differential_test.go)
- [CLI performance guidance](../docs/performance.md)
- [CLI documentation index](../docs/README.md)
- [Contribution guide](../CONTRIBUTING.md)
- [Security policy](../SECURITY.md)
- [Support policy](../SUPPORT.md)
- [License](../LICENSE)
- [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)

This harness belongs only in the Golib engineering inventory. It must not
appear in the consumer package catalog.
