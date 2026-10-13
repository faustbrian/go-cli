# cli documentation map

[Repository README](../README.md) · [Current Golib portal](https://github.com/faustbrian/go-library-tools/blob/main/docs/ecosystem/README.md) · [Published ecosystem snapshot](https://github.com/faustbrian/go-library-tools/blob/v1.8.4/docs/ecosystem/README.md)

This map links package-owned adoption and operational guidance. Start with
the repository README for purpose, installation, and ownership boundaries.
Published compatibility sets are frozen multi-package baselines; they do not
certify every current main revision or a deployed application.

## Module boundaries

Import only the modules needed by your application. A deprecated facade,
preview adapter, or internal harness is not interchangeable with a stable
root API; follow its own README and migration guidance before composing it.

| Module | Status | Owner detail |
| --- | --- | --- |
| `github.com/faustbrian/go-cli` | stable | [README](../README.md) |
| `github.com/faustbrian/go-cli/benchmarks` | internal; non-releasable | [README](../benchmarks/README.md) |

## Choose and adopt

- [Documentation](README.md).
- [Parser engine decision](architecture/parser-engine.md).
- [Architecture and lifecycle](architecture.md).
- [Commands and typed input](commands.md).
- [Compatibility and release policy](compatibility.md).
- [Errors, exit codes, cancellation, and shutdown](errors-and-shutdown.md).
- [`tool` command reference](generated/commands.md).
- [Help, completion, and references](generation.md).
- [Optional ecosystem composition](integrations.md).
- [Intentional limitations](limitations.md).
- [Migration guidance](migrations.md).
- [Mutation testing](mutation.md).
- [Output contract](output.md).
- [Parsing contract](parsing.md).

## Operate and evaluate boundaries

- [Operations and deployment recipes](operations.md).
- [Security](security.md).
- [Threat model](threat-model.md).
- [Troubleshooting and FAQ](troubleshooting.md).

## Maintainer, specification, and evidence references

- [CLI benchmark harness](../benchmarks/README.md).
- [Hardening audit: 2026-07-22](audit/2026-07-22-hardening.md).
- [Benchmark evidence: 2026-07-22 Darwin arm64](benchmarks/2026-07-22-darwin-arm64.md).
- [Benchmark evidence: 2026-09-05 Darwin arm64](benchmarks/2026-09-05-darwin-arm64.md).
- [Performance](performance.md).
- [Release policy](release.md).
- [Source documentation audit](source-documentation-audit.md).

## Related next steps

[Contributor setup and checks](../CONTRIBUTING.md), [Compatibility policy](../COMPATIBILITY.md), [Release history](../CHANGELOG.md), [Private vulnerability reporting](../SECURITY.md), [Support](../SUPPORT.md).
