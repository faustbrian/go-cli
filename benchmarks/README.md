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
benchmark. The construction benchmark and each dispatch iteration include the
following implementation-specific work:

| Case | Timed construction | Timed dispatch iteration |
| --- | --- | --- |
| `go-cli` | Create typed bindings and command definitions, then call `Compile` for validation and immutable runtime preparation. | Recursively rebuild the engine command representation, parse and resolve typed input, run the registered validation plus the normal lifecycle and empty-cleanup path, and invoke the handler. `SetData` marshals the result, creates its human representation, and enforces bounds under synchronization; finalization snapshots that state and marshals the complete `go-cli/v1` envelope. |
| Cobra | Allocate and configure the command tree, register the flag, and link the child. This does not complete runtime setup. | Reset the flag value and `Changed` state, assign argv, and call `ExecuteContext`. The first iteration adds default help, completion, and help-flag state; every iteration performs default-help remove/add work and command-group checks before parsing. Cobra applies `ExactArgs(1)`, the action checks the flag and target, and the action encodes the raw result once. |
| `urfave/cli` | Allocate command and flag definitions without running their operational setup. | Allocate a fresh argv slice and call `Run`. First-run defaults are retained, while command-graph setup runs on every call; parsing then reaches an action that checks the flag, argument count, and target and encodes the raw result once. |
| Kong | Call `kong.New`, including reflection-driven parser and model preparation. | Clear the reusable model, then run Kong's Trace, Reset, Resolve, Apply, and Validate pipeline. The harness checks the populated fields and encodes the raw result once. |
| `flag` | Create a `FlagSet` and register one Boolean flag; there is no command graph. | Check the command token, reset the Boolean value, parse the remaining arguments, validate the flag and positional value, and encode the raw result once in the harness. |

The measurements therefore include different construction phases, validation
locations, first-run effects, lifecycle work, output shapes, and framework
guarantees. Optional lifecycle hooks, cleanup handlers, completion, failure
paths, diagnostics, concurrency, and broader application integration remain
outside this narrow fixture.

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
