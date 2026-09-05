# Performance

Correctness, stable semantics, startup safety, and bounded work outrank
headline benchmarks. The comparative harness measures cold and repeated
invocation of one equivalent observable command contract across `cli`, Cobra,
`urfave/cli`, and Kong.

The root benchmark suite covers small, broad, deep, and maximum construction;
root and deep dispatch; typed conversion; help; completion; manifest
generation; JSON output; success; usage and validation errors; suggestions;
cancellation; and repeated in-process allocation behavior.

Every comparable case selects `deploy`, resolves the same Boolean and
positional values, runs one validation and one action, emits the exact same
`go-cli/v1` JSON envelope, returns success, and leaves stderr empty. The cold
benchmark constructs and invokes a fresh application per iteration. The
repeated benchmark constructs once and exercises sequential reusable
invocation. The conformance test uses the benchmark builders and runners and
must pass before their measurements are compared. It checks success,
validation and structural usage failures, exact stdout and stderr, error
identity, valid-invalid-valid state isolation, the output-size bound, and
incomplete writes through every runner. Cleanup is not part of the compared
command; its owned ordering, failure, and timeout semantics remain covered by
the root suite rather than being attributed to competitors.

Internal algorithms and framework guarantees still differ. See the
[observable contract and execution-path table](../benchmarks/README.md#comparison-boundary)
before interpreting the numbers.

`cli` builds fresh internal parser state to preserve concurrent and repeated
invocation isolation. Cobra, `urfave/cli`, and Kong reuse different mutable
state between iterations. The standard `flag` package remains a separately
reported parsing floor because it has no command graph, action, or output
contract.

The historical targets of four times Cobra latency and 100 `cli` allocations
were based on the former asymmetric fixture and are not regression budgets.
Cross-framework ratios cannot substitute for a stable, current-workspace
`cli` baseline. A future enforced budget requires a controlled runner,
representative corpus, justified threshold, and measured variance.

Run the repository benchmark gate from the repository root:

```sh
make -f verification/package.mk benchmark
```

Use the [benchmark harness commands](../benchmarks/README.md#run) for behavioral
conformance, parser differential coverage, and a retained multi-sample
comparison.

New checked-in evidence must record Go version, behavior-affecting Go settings,
operating system, architecture, CPU, source base and manifest, dependency
versions, comparison-input fingerprint, conformance and benchmark commands,
sample and process count, duration, test and outer process timeouts, raw results,
immutable competitor commits, and pinned statistical method. The capture workflow
compares this identity at both measurement endpoints, validates exact result
cardinality, retains the conformance result plus every sample used by the
statistical report, and removes its task-owned cache trees on exit. Reports
include latency, throughput, bytes, and allocations per invocation.
Results describe that fixture and machine only; no universal speed claim is
made. Review the [benchmark harness boundary](../benchmarks/README.md) before
interpreting cross-library numbers. A materially safer or better-maintained
engine, or a proven regression at the adapter boundary, can reopen the parser
decision.

Current equivalent-contract evidence:
[2026-09-05 Darwin arm64](benchmarks/2026-09-05-darwin-arm64.md).

Historical non-equivalent evidence:
[2026-07-22 Darwin arm64](benchmarks/2026-07-22-darwin-arm64.md).
