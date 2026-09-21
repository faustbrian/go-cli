# Security policy

Report suspected vulnerabilities privately through GitHub Security Advisories
using the [private vulnerability reporting form](https://github.com/faustbrian/go-cli/security/advisories/new).
Do not open a public issue containing an exploit, credential, private endpoint,
or secret-bearing argv.

| Version | Supported |
| --- | --- |
| 1.x | Yes |
| < 1.0 | No |

Security fixes are applied to the current v1 line and released from a supported
v1 revision. A report should identify the affected package, version, input
boundary, observable impact, minimal reproduction, and any known mitigation
without including production credentials or private user data.

## Response process

Critical means a practical confidentiality, integrity, or availability
compromise across ordinary use with ecosystem-wide or irreversible impact.
High means practical authentication, authorization, secret, code-execution,
data-integrity, or severe availability impact in this module. Medium requires
meaningful preconditions or has bounded impact. Low has limited impact or
strong mitigating preconditions.

Maintainers acknowledge Critical reports within one business day, High within
two, Medium within five, and Low within ten. Target remediation is seven
calendar days for Critical issues, 30 days for High, 90 days for Medium, and
the next appropriate release for Low. Targets begin once the report can be
reproduced or confidently bounded; changes and their rationale are communicated
privately to the reporter.

Triage assigns a severity and owner, identifies introduced, affected, and fixed
versions, and records any deferred Medium or Low risk with its rationale,
mitigation, evidence, and review condition. Critical and High findings block an
affected release. Remediation includes a focused regression, affected security
checks, API compatibility where applicable, and an independent final review.

The owner and reporter coordinate an embargo long enough to prepare the
smallest affected-module fix. Access remains need-to-know, and public commits,
CI artifacts, issues, and changelogs omit reporter data, credentials, and
exploit-enabling details. Each affected module is versioned and released
independently with its changelog, upgrade guidance, advisory range, checksums,
SBOM, and provenance as applicable. After fixed artifacts are available, the
owner coordinates advisory publication, reporter credit, and private notice to
affected maintainers. An embargo ends when fixes are broadly available or when
earlier disclosure is necessary to reduce active harm; the private case records
that decision and rationale.

The library never executes a shell, reads environment variables, reads the
working directory, registers process signals, calls `os.Exit`, or starts hidden
goroutines. Applications remain responsible for file, network, configuration,
secret-source, and operating-system policy. The repository threat model and
operational mitigations are in [docs/threat-model.md](docs/threat-model.md).
