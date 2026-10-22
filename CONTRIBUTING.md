# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected package's docs.
2. Run the affected package's deterministic tests as described below.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.

Selected full/release mutation gates must finish with zero surviving viable
mutants; mutation is not an ordinary local or pull-request prerequisite.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Use Go 1.27.3 for verification; the module/API minimum remains Go 1.27.0.
An isolated checkout needs no sibling repositories or coordination goal files.
Begin with deterministic tests and one active target when its argv boundary
changes, using a disposable cache for each bounded session:

```bash
(
  set -eu
  GOCACHE="$(mktemp -d)"
  export GOCACHE
  trap 'rm -rf "${GOCACHE}"' EXIT
  GOWORK=off go test ./...
  GOWORK=off go test . -run '^$' -fuzz '^FuzzRunArgv$' -fuzztime=2s -timeout=2m
)
```

The deterministic command executes fuzz seeds/corpus, not active fuzzing.
Install the verified public v2.4.0 binary archive for your platform from
[go-library-tools releases](https://github.com/faustbrian/go-library-tools/releases/tag/v2.4.0),
checking `checksums.txt` against `.golib.yaml` and the archive against that
checksum set. Add the extracted `golib` binary to `PATH`, or set `GOLIB` to
its path; do not build Tools from source. CI installs and verifies that public
release while checking out its pinned reusable workflow source.

```bash
make inventory
make check
make ci
make specification-check
golib mutation --module . --package .
```

Run adjacent Make commands in one disposable-cache session as shown above.
`make check` and `make ci` run the bounded local contract without service or
test-tag prerequisites in either the root or declared benchmark module. CI
also validates repository/cohesion structure and offline specification
authority. The root race runs once with a two-minute timeout and checks
concurrent repeated signals force shutdown, graceful close releases its
context, and late signals do not reopen its lifecycle. Base-free checks run
seeds/corpus; changed-source CI selects causal active fuzz smoke. Optional
mutation is limited to one package, one worker and a twenty-minute deadline.

Select full qualification, generated documentation and release rehearsal
explicitly:

```bash
make check-full
make -f verification/package.mk docs
make -f verification/package.mk benchmark
make specification-check-online
golib api check
golib release dry-run --module .
```

Full checks retain all nine declared fuzz targets at 2s/2m each (including
`internal/engine`), the repeated concurrency/completion suite, nested
benchmarks and all original module gates. Generated-documentation checking
runs the reference generator, detects drift, checks Bash and pinned Zsh 5.9
completion syntax and rejects shell evaluation; its release-source guard is
unchanged. Online specification checking fetches authority/errata sources when
applicable. Release rehearsal can run locally. Report unavailable or failing
commands; bounded checks alone do not establish release qualification.

## Shared Tooling

Local verification uses the public `golib` release and checksum declared in
`.golib.yaml`. The repository-owned `verification/package.mk`
contains only the nested benchmark and generated-documentation checks that
cannot be represented by the shared tool's direct operations.

Mutation checkpoints under `.verification/mutation/` are source-specific
evidence. Do not rerun an approved campaign solely because repository history
or tooling paths changed; the shared tool reuses it only when its complete
content and verifier identities match.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.

## Source Documentation

Install the public `golib` v2.4.0 verifier declared in `.golib.yaml` and
validate its declared checksum before repository checks. Run
`make source-docs-check` for the tracked-source documentation contract.
`make check`, `make check-full` and `make ci` require the same strict check.
Required CI also checks all tracked source, including inactive build variants.
Manual-review findings stay visible; structural success does not certify prose
truth, specification conformance or generator reproducibility.
