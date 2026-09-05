# CLI benchmark harness

This internal, non-releasable module measures a matched command fixture across
`go-cli`, Cobra, `urfave/cli`, Kong, and the standard `flag` package. The
implementations do not perform identical setup or output work, so the numbers
must be interpreted with the differences below. The module also preserves
differential parser evidence against the former Cobra-backed implementation.

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

The cases share the `deploy --force target` fixture and check the same Boolean
and positional values. This is a matched scenario, not an equivalent-work
benchmark:

- `go-cli` construction creates bindings and command definitions and calls
  `Compile`, including its validation and immutable runtime preparation.
- Cobra construction allocates its command tree, registers the flag, and links
  the child command.
- `urfave/cli` construction allocates command and flag definitions but defers
  operational preparation until `Run`.
- Kong construction includes `kong.New` and its reflection-driven parser and
  model preparation.
- `flag` construction creates a `FlagSet` and registers one Boolean flag; it
  has no command graph.

Dispatch reuses the value prepared before the timed loop, but each case still
performs different work:

- `go-cli` parses and resolves typed input, runs the registered validation and
  its normal lifecycle and cleanup pipeline, and invokes the handler. `SetData`
  marshals the result, creates its human representation, and enforces output
  bounds under synchronization. Finalization snapshots that state and marshals
  the complete `go-cli/v1` response envelope.
- Cobra parses the command and flag, applies `ExactArgs(1)`, validates the flag
  and target in the action, and encodes the raw result once.
- `urfave/cli` performs its deferred preparation and parsing in `Run`, validates
  the flag, argument count, and target in the action, and encodes the raw result
  once.
- Kong parses into its model, after which the harness validates the fields and
  encodes the raw result once.
- `flag` checks the command token in the harness, parses the remaining
  arguments, validates the flag and positional value in the harness, and
  encodes the raw result once.

The measurements therefore include different construction phases, validation
locations, lifecycle work, output shapes, and framework guarantees. Optional
lifecycle hooks, cleanup handlers, completion, failure paths, diagnostics,
concurrency, and broader application integration remain outside this narrow
fixture.

The differential parser test compares the owned parser with the former Cobra
adapter across options, aliases, nesting, negative values, help, version, and
failure categories. It is compatibility evidence, not a performance result.

Do not infer a universal framework ranking or a like-for-like efficiency ratio
from the cross-library numbers. Measure the actual command shape, output mode,
lifecycle, concurrency, and correctness requirements of the application being
built.

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
