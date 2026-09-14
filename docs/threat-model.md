# Threat model

Version: `CLI-TM-2.0`

Owner: go-cli maintainers

## Scope and assets

This model covers the planned, unpublished v2 source line. The owned boundary
is command construction, argv parsing, typed input,
lifecycle dispatch, buffered output, completion, generated references, and
stable errors. Assets are secret-marked input values, terminal and JSON output
integrity, bounded process memory and CPU, lifecycle ordering, cancellation,
and error-classification compatibility.

The core does not own operating-system argv exposure, shell history, caller
logs or telemetry, direct human-mode IO, files, networks, databases, secret
stores, subprocesses, panics, or process termination.

## Trust boundaries

| Boundary | Trust assumption | Owned control |
| --- | --- | --- |
| argv and completion input | hostile and resource-amplifying | UTF-8, NUL, count, byte, graph, suggestion, completion-inspection, accepted-result, and response-byte limits |
| command definitions | application-controlled but fallible | compile-time shape, identity, metadata, and cardinality validation |
| validators, middleware, handlers, and hooks | trusted application code with access according to the callback contract | context propagation, ordering, stable classification, secret-aware cause protection |
| structured output values and types | application-controlled and potentially very large | pre-serialization value/type graph, collection-count, metadata, and byte bounds; custom serializers rejected |
| stdout and stderr writers | caller-owned and fallible | mode isolation, complete-write checks, bounded buffered payloads, output classification |
| operating system and external services | outside the library | explicit application ownership and context-bearing callback APIs |

## Material threats and mitigations

- **Secret disclosure through diagnostics:** parser-owned secret conversion
  errors omit rejected values. When a selected command has any secret binding,
  callback, completion, cancellation, and output-writer causes are replaced by
  a protected cause that preserves `errors.Is` matching without exposing error
  text or concrete cause values. Before selection completes, cancellation and
  render causes receive that protection when any command in the compiled
  application accepts secrets. Direct completion preserves protected
  cancellation-cause identity under the same rule.
- **Memory or CPU amplification through output:** `SetData` bounds the
  reflection graph and worst-case built-in serialization size before invoking
  JSON or human formatting. Application-defined marshaler, text-marshaler,
  stringer, formatter, error, and `json:",omitzero"` `IsZero` callbacks are
  rejected because the runtime cannot constrain work performed inside them.
  Raw JSON estimates include the encoder's HTML and line-separator escape
  expansion, and decisive raw-size failures return before scanning rejected
  content. Encoder-reachable type depth, count, and struct metadata are bounded
  even behind nil containers, and JSON tag validation mirrors the standard
  encoder's field-name fallback.
- **Terminal or protocol injection:** owned messages, generated Markdown, and
  completion fields strip terminal controls and keep single-line protocol
  fields bounded. Provider fields must fit the remaining raw response budget
  before sanitization traverses them.
- **Lifecycle races and lost failures:** middleware continuation ownership,
  synchronous reverse cleanup, context checks between phases, and joined
  primary/cleanup failures preserve ordering and classification without hidden
  goroutines.

## Accepted risks

| ID | Severity | Owner | Risk and rationale | Current mitigation | Review condition |
| --- | --- | --- | --- | --- | --- |
| CLI-R1 | Medium | go-cli maintainers | A cleanup hook that ignores its context can block `Run` indefinitely. Go cannot safely terminate an arbitrary in-process function, and detaching it would leak the goroutine and captured resources. | Hooks are trusted application code, run synchronously in reverse order, receive a default 30-second deadline, and must honor `ctx.Done()`. Untrusted cleanup belongs behind an application-owned process boundary. | Revisit if callback isolation is redesigned, a process-isolated cleanup API is proposed, or an incident shows hooks routinely ignore cancellation. |
| CLI-R2 | Medium | application owner | Secret argv can remain visible to the operating system, shell history, parent processes, CI metadata, container APIs, task overrides, and crash reports. The library receives already-tokenized argv and cannot erase upstream copies. | Prefer caller-owned stdin, protected files, or an explicit secret provider; keep secret bindings marked so owned diagnostics and metadata redact them. | Revisit if the API gains an owned secret-input channel or a supported platform provides reliable argv scrubbing. |
| CLI-R3 | Medium | application owner | Handlers and hooks can expose secrets through direct IO, logs, telemetry, panics, or returned non-secret-command errors because they are trusted in-process code outside the presentation boundary. Intercepting all application effects would require hidden global IO or recovery policy. | Secret-bearing commands protect callback causes; applications must use invocation IO, sanitize private diagnostics, and install explicit safe panic recovery when required. | Revisit after a disclosure incident, a callback capability redesign, or addition of an owned observability or recovery boundary. |
| CLI-R4 | Medium | application owner | Callback-owned file, network, database, queue, retry, idempotency, and timeout work can remain unbounded or SSRF-, traversal-, injection-, replay-, or partial-failure-prone. The core has no authority over resources deliberately opened by application callbacks. | Every callback receives a context; applications must validate hostile values, set explicit resource limits and deadlines, use safe protocol primitives, and own cleanup and idempotency. | Revisit when the core adds an external-resource adapter or an incident shows the callback contract is insufficiently explicit. |
| CLI-R5 | Low | go-cli maintainers | A compromised dependency, workflow action, release credential, or maintainer can alter source or artifacts despite correct runtime controls. This is not preventable solely by the library API. | The root module has no runtime dependencies; CI and release policy require immutable action/tool pins, review, secret scanning, vulnerability checks, checksums, SBOM, provenance, and signatures at the applicable release boundary. | Revisit on dependency or workflow changes, maintainer-access changes, release-key rotation, or any supply-chain incident. |

There are no known unresolved Critical or High findings in this reviewed source
boundary. `CLI-R1` through `CLI-R4` are accepted Medium risks with explicit
ownership and application-facing mitigations; `CLI-R5` is accepted Low risk.

## Compatibility and release disposition

The stable published line remains v1.0.1. Rejecting custom structured-output
serializers and withholding concrete secret-command callback causes are
intentional public behavior changes isolated behind the `/v2` module path.
This v2 source is planned and non-releasable until its API baseline, direct
owned consumers, security gates, and publication evidence are complete. The
repository contains no local replacement directive.
