# Security

Mark secret arguments and options with `Secret`. Framework diagnostics,
conversion causes, metadata, manifests, help defaults, middleware, completion,
and JSON envelopes omit secret values. Dynamic completion on secret bindings is
rejected. Declared enum values on secret bindings are also omitted from option
metadata, Markdown, manifests, and static completion. Tests should walk
complete error chains and assert known secrets are absent.

When a selected command declares a secret binding, validation, completion,
middleware, handler, pre-run, post-run, cleanup, cancellation, and output-writer
failures use a protected cause. The public error chain retains `errors.Is`
identity while withholding application error text and concrete causes from
`errors.As`. This conservative rule applies even when a callback did not
include the secret in its error. Before command selection completes,
cancellation and render causes receive the same protection when any command in
the compiled application declares a secret binding. Direct completion retains
protected cancellation-cause identity under the same rule.

Command-line secrets may still be visible to operating-system process
inspection, shell history, CI logs and metadata, container APIs, ECS task
overrides, crash reports, and parent processes. Prefer caller-owned stdin,
application-controlled files, or an explicit secret provider. The framework
does not read environment variables or secret stores.

Framework diagnostics and completion descriptions strip terminal controls,
including ANSI, OSC, carriage return, Unicode bidi controls, and newlines where
a single-line protocol is required. Generated Markdown applies the same
control filtering. JSON rendering never introduces ANSI.
Generated completion invokes the executable with quoted token arrays or parsed
PowerShell AST values. It never reconstructs and evaluates a command string;
shell metacharacters in partial input and candidates remain literal data.
Applications remain responsible for sanitizing their own direct IO and logs.

Argv count and bytes, graph depth and breadth, aggregate metadata bytes,
definitions, completion-provider candidate inspection, accepted completion
count and bytes, output records and bytes, structured output collection count
and traversal, encoder-reachable type depth, count, and metadata, and suggestion
work are bounded.
`Output.SetData` rejects application-defined `json.Marshaler`,
`encoding.TextMarshaler`, `fmt.Stringer`, `fmt.Formatter`, and `error`
implementations, plus `IsZero` methods selected by `json:",omitzero"`, because
an in-process callback can allocate or block before returning bytes. Invalid
UTF-8 and NUL are rejected. Completion providers receive hostile partial input
and must validate it before any deliberate external access. Provider candidates
must fit the remaining raw byte budget before terminal sanitization runs.
Structured-output tag metadata is bounded before parsing, and invalid JSON tag
names use the same field-name fallback as the standard encoder.

The core never invokes a shell, expands a path, evaluates command text, opens a
file, uses `unsafe`, uses cgo, links runtime internals, reads current working
directory, calls `os.Exit`, registers signals, or starts hidden goroutines.
Commands own file, database, network, retry, idempotency, and TOCTOU policy.

Panic recovery is not enabled by default. Explicit recovery middleware must
avoid formatting secret-bearing values and should retain a safe diagnostic plus
an application-private stack trace.

See [the repository threat model](threat-model.md) for assets, trust
boundaries, mitigations, and accepted risks.
