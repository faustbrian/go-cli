# Compatibility and release policy

The module follows Semantic Versioning. Public exported identifiers, typed
binding semantics, parser forms, Unicode policy, generated schemas, output
envelopes, error kinds and sentinels, default exit codes, completion protocol,
and deterministic ordering are compatibility contracts.

The minimum supported and CI-tested toolchain is Go 1.27.0. Releases run with
`GOWORK=off`; local workspaces must not hide missing module dependencies. The
exported API is compared with its checked-in baseline.

The parser is owned behind `internal/engine` and has no runtime module
dependencies. Parser changes require differential argv tests, help and
completion drift review, benchmarks, and a changelog entry. Internal parser
errors and types are not public contracts.

Every user-visible addition, compatibility decision, security fix, deprecation,
and breaking change enters `CHANGELOG.md` under Unreleased before release.
Generated artifacts are byte-compared in CI. Releases include reproducible
archives, checksums, SBOM, provenance, and signatures.

Deprecations include a message and replacement path in help and manifests.
Removal waits for a breaking release unless the behavior is unsafe. Security
corrections may deliberately tighten hostile-input acceptance and will be
documented with migration guidance.

## Unreleased API transition

The planned `github.com/faustbrian/go-cli/v2` line rejects application-defined
structured-output serializers and `json:",omitzero"` `IsZero` callbacks whose
execution or allocation cannot be bounded. It also rejects encoder-reachable
type graphs or struct metadata that exceed the documented output boundary.
It also withholds concrete callback causes from `errors.As` when the selected
command declares any secret binding, while retaining `errors.Is` identity;
preselection cancellation and render failures use the same protection when any
compiled command accepts secrets. Direct completion also retains protected
cancellation-cause identity. These changes intentionally tighten released v1
behavior and therefore remain isolated behind the unpublished v2 module path.
Existing consumers must stay on v1.0.1 until v2 is published, then replace
custom `Output.SetData` values with bounded built-in values and avoid relying
on concrete callback-cause recovery for commands that accept secrets.
