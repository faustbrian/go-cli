# CLI benchmark harness

This internal, non-releasable module measures one equivalent observable command
contract across `go-cli`, Cobra, `urfave/cli`, and Kong. The standard `flag`
package is retained separately as a parsing floor because it does not own a
command graph. The module also preserves differential parser evidence against
the former Cobra-backed implementation.

The module exists only for Golib engineering verification. It has no supported
installation path, public package, semantic-version release, or runtime
dependency relationship with consumers. Applications should depend on the
public [`go-cli` module](../README.md), not this harness.

The harness requires Go 1.27.0 or newer.

## Run

From the repository root, create one disposable root and use the repository
workspace to compare the current root source:

```console
evidence_root="$(mktemp -d "${TMPDIR:-/tmp}/go-cli-benchmark.XXXXXX")"
GOCACHE="${evidence_root}/gocache" \
GOMODCACHE="${evidence_root}/gomodcache" \
GOTMPDIR="${evidence_root}/tmp" \
GOTOOLCHAIN=go1.27.0 \
  ./scripts/capture-benchmark-evidence.sh "${evidence_root}/evidence"
```

The root module's benchmark gate runs the same comparison through the
checked-in workspace. The capture command runs the exact conformance tests
inside the captured identity window, requires an absolute task-owned path
outside the repository, and refuses to overwrite it. It records endpoint
identity around conformance and the benchmark, retains the conformance result,
source manifest, and ten raw samples from ten separate benchmark processes,
then writes the pinned statistical
summary to `benchstat.txt`, separated by comparison class. The endpoint identity includes source state,
toolchain, behavior-affecting Go environment, machine and power mode,
dependencies, immutable competitor revisions, fixture, and method; matching
endpoints detect persistent input drift during the run. Every Go subprocess has
a 120-second outer process-group watchdog, and test binaries have an additional
110-second internal timeout. The capture rejects missing or unexpected
benchmark rows. Its exit trap removes partial staging and all three initially
empty task-owned Go cache trees.
Publish `capture.txt`, `identity.txt`, `source-manifest.txt`,
`conformance.txt`, the raw samples, and the summary together. The script removes
its disposable caches before atomically publishing that complete directory.
Report latency, invocations per second, bytes and allocations per invocation,
and the confidence intervals. Do not publish only the fastest run. Comparative
captures are non-gating engineering evidence unless their runner and noise
controls are explicitly qualified for a stronger claim.

## Comparison boundary

Every equivalent case accepts the `deploy --force target` fixture, selects a
native `deploy` subcommand, resolves `force=true` and the single positional
`target`, runs one validation and one action, returns success with no stderr,
and writes exactly this versioned result:

```json
{"schema":"go-cli/v1","ok":true,"data":{"target":"target","force":true}}
```

The conformance tests execute the same builders and runners as the benchmarks.
They prove two sequential successes and a valid-invalid-valid sequence with
fresh output streams for every framework, including exact success,
validation-failure, structural-usage-failure, and output-limit envelopes;
stable error identities; empty stderr; and reset mutable state. They exercise
the one-mebibyte bound and short-writer propagation through every runner. A
benchmark result is comparable only while all of those tests pass.

The cold benchmark constructs and invokes a fresh application in every
iteration. The repeated benchmark constructs once, then invokes the reusable
application in every iteration. This avoids attributing incompatible amounts
of deferred initialization to a separate construction score. Each framework
uses its supported public execution path to provide the same observable
contract:

| Case | Public execution path and implementation-owned work |
| --- | --- |
| `go-cli` | `Compile` validates and snapshots the typed command graph. `RunCommand` builds invocation-local parser state, resolves typed input, runs validation and lifecycle, records bounded structured output, and renders the complete envelope. |
| Cobra | `ExecuteContext` initializes its command machinery, resolves the command and PFlag value, applies `ExactArgs`, runs `PreRunE` validation and `RunE`, then uses the shared bounded output adapter to render the same envelope. The harness disables the optional public completion command. |
| `urfave/cli` | `Run` performs its framework-owned setup and parsing, reaches the selected command's `Before` validation and `Action`, then uses the shared bounded output adapter to render the same envelope. |
| Kong | `kong.New` prepares the reflection-driven model. `Parse` resets, resolves, applies, and validates it; `Context.Run` invokes the selected command, then the shared bounded output adapter renders the same envelope. |

All four paths consume allocation-free views of the same complete process argv.
For the three competitors, the shared output adapter performs the same result
JSON encoding, human rendering, one-mebibyte bound, invocation-local buffering,
snapshot copy, envelope encoding, and complete-write check as the owned JSON
path. These are observation adapters needed to equalize the public result; no
counter, retained test state, or sink exists solely to make a timed iteration
appear complete.

Competitor source identity is pinned separately from module version identity in
[`competitors.txt`](competitors.txt). The capture records both values in its
endpoint identity and fingerprints that file with the measured source.

Equivalent behavior does not imply identical internal algorithms or guarantees.
The measured cost includes each framework's required work for this contract.
Optional hooks, completion, diagnostics, concurrent invocation, and broader
application integration remain outside the fixture. Validation failure is a
conformance path but is not timed. Lifecycle cleanup is not part of this
command, so this benchmark does not compare framework cleanup APIs; the root
suite separately verifies `go-cli` cleanup order, failure composition, and
bounded cleanup context.

`BenchmarkParsingFloorFlag` parses only `--force target` with a reusable
`flag.FlagSet`. It has no subcommand selection, action, or output contract and
MUST NOT appear in equivalent-framework ratios or regression budgets.

The differential parser test compares the owned parser with the former Cobra
adapter across options, aliases, nesting, negative values, help, version, and
failure categories. It is compatibility evidence, not a performance result.

Do not infer a universal framework ranking from this one contract. Measure the
actual command shape, output mode, lifecycle, concurrency, and correctness
requirements of the application being built.

## Navigation

- [Benchmark source](compare_test.go)
- [Parser differential evidence](parser_differential_test.go)
- [CLI performance guidance](../docs/performance.md)
- [Current equivalent-contract evidence](../docs/benchmarks/2026-09-05-darwin-arm64.md)
- [CLI documentation index](../docs/README.md)
- [Contribution guide](../CONTRIBUTING.md)
- [Security policy](../SECURITY.md)
- [Support policy](../SUPPORT.md)
- [License](../LICENSE)
- [Golib ecosystem index](https://github.com/faustbrian/go-library-tools/blob/v1.4.0/docs/ecosystem/README.md)

This harness belongs only in the Golib engineering inventory. It must not
appear in the consumer package catalog.
