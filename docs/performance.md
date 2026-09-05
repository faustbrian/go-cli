# Performance

Correctness, stable semantics, startup safety, and bounded work outrank
headline benchmarks. The comparative harness measures construction and
prepared dispatch separately for one matched command fixture. It is not an
equivalent-work comparison: each library performs implementation-specific
setup, validation, and output work.

The root benchmark suite covers small, broad, deep, and maximum construction;
root and deep dispatch; typed conversion; help; completion; manifest
generation; JSON output; success; usage and validation errors; suggestions;
cancellation; and repeated in-process allocation behavior.

Construction includes `cli.Compile` and `kong.New`; the Cobra, `urfave/cli`,
and `flag` cases allocate definitions but defer different amounts of work.
Dispatch is not uniformly prepared. `cli` rebuilds an engine command before
fresh parsing and then runs typed validation, its normal lifecycle and cleanup
path, `SetData`, and complete envelope rendering. Cobra resets flag state and
argv, then performs first-run and repeated `ExecuteContext` setup before its
argument check, action validation, and raw-result encoding. `urfave/cli`
allocates argv on every iteration, retains first-run defaults, rebuilds command
graph state on every `Run`, and validates and encodes in its action. Kong clears
its model and runs Trace, Reset, Resolve, Apply, and Validate before harness
validation and raw-result encoding. The `flag` case resets its value before
parsing, harness validation, and raw-result encoding.

Every case writes to `io.Discard`, but setup persistence, validation location,
lifecycle work, encoded shape, and framework guarantees differ. See the
[per-case timed-work table](../benchmarks/README.md#comparison-boundary) before
interpreting the numbers.

`cli` dispatch builds fresh internal parser state to preserve concurrent and
repeated invocation isolation. Cobra, `urfave/cli`, Kong, and `flag` reuse
different mutable state between iterations. The standard `flag` package
remains a parsing floor because it has no command graph.

The previously documented targets of four times Cobra latency and 100 `cli`
allocations are not enforced by the repository benchmark gate. The standalone
`scripts/check-benchmark-budget.sh` audit also uses `GOWORK=off`, so it measures
the nested module's pinned public `go-cli` dependency rather than current root
source. Treat those values as historical, unenforced targets until a
current-workspace regression gate is deliberately defined.

Run the repository benchmark gate from the repository root:

```sh
make -f verification/package.mk benchmark
```

Use the [benchmark harness commands](../benchmarks/README.md#run) for the
matched-fixture differential test and a retained multi-sample comparison.

Checked-in evidence records Go version, operating system, architecture, commit,
fixture hash, benchmark command, raw results, and statistical comparison.
Results describe that fixture and machine only; no universal speed claim is
made. Review the [benchmark harness boundary](../benchmarks/README.md) before
interpreting cross-library numbers. A materially safer or better-maintained
engine, or a proven regression at the adapter boundary, can reopen the parser
decision.

Historical checked-in evidence: [2026-07-22 Darwin arm64](benchmarks/2026-07-22-darwin-arm64.md).
