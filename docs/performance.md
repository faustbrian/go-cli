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

Construction includes `cli.Compile`, Cobra's command and flag registration,
and `kong.New`. The `urfave/cli` construction case allocates definitions while
deferring operational preparation until `Run`; standard `flag` has no command
graph. During dispatch, `cli` uses `SetData` and renders its complete
`go-cli/v1` envelope. Cobra and `urfave/cli` encode a raw result from their
actions, while Kong and `flag` validate in the harness and encode the raw
result there. Every case writes to `io.Discard`, but the encoded shapes and
framework work differ.

Prepared `cli` dispatch builds fresh internal parser state to preserve
concurrent and repeated invocation isolation. Direct Cobra dispatch reuses its
mutable prepared graph. The release budget permits up to four times that Cobra
latency and 100 allocations for this fixed harness. That threshold is a
project regression guardrail, not an equivalent-work ratio or a universal
performance claim. Standard `flag` remains a parsing floor because it has no
command graph.

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

Current checked-in evidence: [2026-07-22 Darwin arm64](benchmarks/2026-07-22-darwin-arm64.md).
