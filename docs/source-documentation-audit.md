# Source documentation audit

[Documentation map](index.md) · [Repository README](../README.md)
[Next: Operations and deployment recipes](operations.md)

## Contract and scope

Tier A source documentation from `3c744bdd6fc6541f2d9c02b498c4d271b54952b3`.
Worker owns canonical package/API comments and adjacent implementation rationale,
with only directly contradicted maintained-document corrections. Preserve all
executable tokens, compiler/analyzer/security directives, dependency inputs,
example outputs, API declarations and historical benchmark evidence. Parent owns
shared checker adoption, delivery and exact-source CI. No runtime or terminal UI
changes, browser campaign or new release are included.

## Reproducible inventory

```sh
git ls-files '*.go' 'go.mod' '**/go.mod'
cat modules.json packages.json .golib.yaml .go-version
rg -n '^package |^type |^func |^const |^var ' --glob '*.go'
rg -n 'go:generate|Code generated|go:build|\+build|TODO|FIXME|HACK|NOTE|WARNING|Deprecated:|nolint|nosec|lint:ignore' --glob '*.go'
golib source-docs inventory --json
golib source-docs check --json
```

Root public module and nested `benchmarks` module; catalog Go minimum 1.27.0
and producer pin 1.27.2. Root package `cli`, public test support `clitest`, private
parser `internal/engine`, repository fixtures `internal/referenceapp`, generator
`cmd/generate-reference`, process fixture `cmd/process-fixture`, and test-only
benchmark package are all in scope. External test packages are reviewed alongside
the owning package. No build/platform variants or generated Go are present.
Existing security/lint directives and generated-reference provenance are retained.
The deprecation strings in generation/tests are rendered command metadata, not
Go declaration deprecation markers. No actionable unfinished marker was found.

| Tracked Go file | Package | Lines at base | Initial contract anchors |
| --- | --- | ---: | --- |
| `api_compatibility_test.go` | `cli_test` | 181 | TestPublicAPICompatibility, publicAPI, exportedDeclarations, exportedSpecification, exportedFields and further contracts |
| `architecture_test.go` | `cli_test` | 62 | TestArchitectureKeepsEngineAndRuntimeDetailsInternal |
| `argument.go` | `cli` | 191 | StringArgument, StringsArgument, IntArgument, UintArgument, FloatArgument and further contracts |
| `benchmark_test.go` | `cli_test` | 228 | BenchmarkCompileSmallTree, BenchmarkCompileLargeTree, BenchmarkCompileShapes, BenchmarkDispatch, BenchmarkDeepDispatch and further contracts |
| `benchmarks/compare_test.go` | `benchmarks` | 631 | Error, Unwrap, Is, validateComparison, comparisonResultForTarget and further contracts |
| `benchmarks/parser_differential_test.go` | `benchmarks` | 356 | TestOwnedParserMatchesTheFormerCobraAdapter, differentialCommand, differentialResultsEqual, parseWithFormerCobraAdapter, parseCobra and further contracts |
| `clitest/example_test.go` | `clitest_test` | 32 | TestExampleRun |
| `clitest/harness.go` | `clitest` | 125 | WithStdin, WithOutput, WithNonInteractive, Run, RunContext and further contracts |
| `clitest/harness_test.go` | `clitest_test` | 59 | TestHarnessExecutesWithoutProcessGlobalMutation, TestHarnessAssertsClassifiedFailures |
| `cmd/generate-reference/main.go` | `main` | 68 | main, generate |
| `cmd/process-fixture/main.go` | `main` | 76 | main |
| `command.go` | `cli` | 232 | NewCommand, AddSubcommands, WithAliases, WithSummary, WithVersion and further contracts |
| `command_set.go` | `cli` | 494 | CompileCommandSet, RunCommand, runParsed, commandSetCommand, withExitCode and further contracts |
| `command_set_test.go` | `cli_test` | 572 | TestCommandSetPreservesBoundedCommandHelpVersionAndErrors, TestCommandSetParsesTypedCommandOptions, TestCommandSetOptionParsingPreservesCancellationAndBoundedVersionOutput, TestCommandSetRejectsInvalidOrAmbiguousDefinitions, TestCommandSetBoundsInvocationAndHandlerFailures and further contracts |
| `command_test.go` | `cli_test` | 199 | TestNewCommandIgnoresNilOptions, TestCompilePublishesAnImmutableOrderedCommandGraph, TestCompileRejectsInvalidCommandGraphs, commandNames, equalStrings |
| `compile.go` | `cli` | 962 | WithLimits, WithExitCodePolicy, Compile, compileCommand, defaultLimits and further contracts |
| `completion.go` | `cli` | 365 | Complete, completionAttachedShortOption, enumCandidates, completionPosition, completionAssignedOption and further contracts |
| `completion_test.go` | `cli_test` | 334 | TestCompletionCombinesStaticAndExplicitDynamicCandidates, TestCompletionBoundsInspectedProviderCandidates, TestCompletionRejectsRawOversizeBeforeSanitization, TestCompletionProvidesNonSecretEnumValues, TestCompletionIsBoundedAndCancellationAware and further contracts |
| `contracts_internal_test.go` | `cli` | 331 | TestTypedDeclarationsExposeEverySupportedContract, TestScalarParserPreservesExactTokensAndFailures, TestCompilationRejectsEveryMalformedDeclarationClass, TestCompileOptionsAndMetadataBoundaries, TestCommandSetCompilationTracksOptionDispatch and further contracts |
| `doc.go` | `cli` | 9 | Canonical package contract |
| `docs_test.go` | `cli_test` | 58 | TestRequiredDocumentationSetIsPresentAndSubstantive |
| `errors.go` | `cli` | 209 | Error, GoString, Is, Kind, Error and further contracts |
| `example_test.go` | `cli_test` | 54 | ExampleApplication_Run, ExampleApplication_Run_json |
| `fuzz_test.go` | `cli_test` | 289 | FuzzRunArgv, FuzzCompileCommandGraph, FuzzTypedConversion, FuzzCompletionPartialArgv, FuzzLifecycleFailureAndCancellation and further contracts |
| `generation.go` | `cli` | 529 | Help, wrapHelp, wrapHelpLine, ManifestJSON, Markdown and further contracts |
| `generation_test.go` | `cli_test` | 475 | TestHelpIsGeneratedFromCompiledMetadata, TestManifestAndMarkdownAreDeterministic, TestMarkdownStripsControlsFromAllMetadata, TestManifestRejectsANilApplication, TestEnumContractsAreValidatedAndPublished and further contracts |
| `groups_test.go` | `cli_test` | 166 | TestRequiredAndGroupedOptionsValidateBeforeSideEffects, TestDefaultsCanSatisfyRequiredOptions, TestCompileRejectsInvalidGroupsAndReusedBindings, TestCompileRejectsUnsatisfiableOptionGroups, TestCompileAcceptsAnOptionalTogetherExclusiveComponent |
| `internal/engine/engine.go` | `engine` | 454 | Completion, generateCompletion, completionFunction, posixQuote, fishQuote and further contracts |
| `internal/engine/engine_test.go` | `engine` | 227 | TestCompletionSupportsEveryDocumentedShell, TestParseBuildsFreshTreesAndClassifiesTerminalActions, TestParseErrorMessagesRemainEngineLocal, TestNegativeValueRecognition, TestVersionRecognitionStaysAtConfiguredRootBoundary and further contracts |
| `internal/engine/fuzz_test.go` | `engine` | 42 | FuzzAdapterTranslation |
| `internal/referenceapp/reference.go` | `referenceapp` | 42 | New |
| `lifecycle_test.go` | `cli_test` | 500 | TestLifecycleRunsInDeterministicOrder, TestLifecycleFailureSemantics, TestCleanupFailureDoesNotErasePrimaryFailure, TestRequiredInteractionFailsBeforeSideEffects, TestCancellationBetweenLifecyclePhasesStopsSideEffects and further contracts |
| `limits_test.go` | `cli_test` | 116 | TestCompileBoundsCommandTreeDepthAndBreadth, TestApplicationUsesConfiguredArgvLimits, TestZeroLimitFieldsUseAuditableDefaults, TestCompileOptionsComposeNonZeroOverrides, TestCompileBoundsMetadataBytes |
| `mutation_contracts_internal_test.go` | `cli` | 1417 | TestCompilationExactBoundariesAndAllocatedIdentities, TestOptionGroupSatisfiabilityTraversesEveryComponent, TestASCIIAlphaNumericIncludesOnlyExactRanges, TestCompletionPositionPreservesTokenGrammar, TestCompletionArgumentAndCandidateBoundsAreExact and further contracts |
| `option.go` | `cli` | 226 | StringOption, BoolOption, IntOption, UintOption, FloatOption and further contracts |
| `output.go` | `cli` | 778 | Info, SetData, validateOutputValue, estimateOutputSize, estimateOutputSizeWithTypes and further contracts |
| `output_test.go` | `cli_test` | 374 | TestHumanOutputSeparatesSuccessAndErrors, TestJSONOutputUsesVersionedDeterministicEnvelopes, TestMachineAndQuietModesIsolateDirectHandlerWrites, TestQuietSuppressesSuccessButStillRendersErrors, TestWriterFailureIsAStableOutputError and further contracts |
| `parsing_test.go` | `cli_test` | 193 | TestTypedArgumentsConvertBeforeHandlerExecution, TestMalformedTypedInputIsClassifiedBeforeSideEffects, TestSecretConversionErrorsContainNoSecretInTheirErrorChain, TestArgvRejectsInvalidUTF8AndOversizedInput, TestCancellationCauseIsPreserved and further contracts |
| `process_test.go` | `cli_test` | 119 | TestExecutableBoundaryMapsOutputStatusAndTermination, runProcessFixture, assertProcessEnvelope |
| `requests_test.go` | `cli_test` | 232 | TestHelpAndVersionRequestsAreSuccessfulTypedResults, TestRunCommandPreservesCommandsButDoesNotPublishCompletionProtocol, TestRunCommandPreservesInvocationValidation, TestParserFailuresHaveStableSpecificClassifications, TestParserStopsAtTheFirstUnknownCommand and further contracts |
| `run.go` | `cli` | 1150 | Input, IO, Interactive, Output, Run and further contracts |
| `run_test.go` | `cli_test` | 273 | TestRunParsesTypedInputWithoutExposingEngineState, TestRunHonorsDoubleDashAndOptionPlacementAfterArguments, TestRegisteredDigitShorthandWinsOverNegativePositionalSyntax, TestRunUsesConfiguredExitCodePolicy, TestCompileRejectsInvalidExitCodePolicy and further contracts |
| `runtime_internal_test.go` | `cli` | 495 | TestErrorAndExitContractsCoverNilAndEveryClassification, TestOutputBoundariesAreExplicitAndWriterSafe, TestValueConversionFailuresAndDefensiveCopies, TestGenerationAndCompletionDefensiveBoundaries, TestRunDefensiveAndHeadlessBoundaries and further contracts |
| `security_test.go` | `cli_test` | 708 | TestSecretCallbackFailuresOmitSensitiveValues, TestSecretArgumentProtectsHandlerFailure, TestProtectedCauseGoSyntaxOmitsValueTypedSecrets, TestSecretCommandProtectsOutputWriterFailure, TestSecretCommandProtectsCancellationCause and further contracts |
| `shutdown.go` | `cli` | 106 | NewShutdownController, Context, Forced, Signal, Close |
| `shutdown_test.go` | `cli_test` | 95 | TestShutdownControllerSupportsGracefulThenForcedPolicy, TestShutdownControllerIsConcurrentAndNilSafe, TestShutdownControllerCloseReleasesGracefulContext |
| `value.go` | `cli` | 164 | bindingValue, bindingState, cloneValue, last, parseString and further contracts |

## Gaps and investigation questions

- Canonical package docs must distinguish full command-tree execution from the
  deliberately bounded command-set surface, stdlib dependencies, borrowed IO and
  callbacks, and application-owned shutdown/terminal interaction.
- Audit every exported declaration, field and interface method. Missing source
  contracts are expected around option/argument aliases, callback lifecycle,
  invocation/output ownership, error redaction and shutdown context lifetime.
- Investigate before promising copy isolation or concurrent reuse: mutable
  definitions vs compilation; generic default/value aliasing; compiled bindings;
  caller callbacks/streams and invocation-local state.
- Trace limits before allocations, conversion/redaction boundaries, sentinel
  errors/partial results, middleware/context lineage, cleanup order and detached
  cleanup deadlines, completion provider trust/output caps, and bounded JSON/
  terminal sanitization. Do not infer cancellation of blocking callbacks or IO.
- Inspect all tests/fuzz/helpers/benchmarks: identify their actual parser, lifetime,
  error/output and differential-model boundaries rather than claiming live-shell
  or production performance from fixtures.
- Inspect `.golib.yaml`, API test and generator scripts separately. Apidiff is
  configured; repository-native structural oracle/reference generation may have
  distinct requirements. Refresh only genuinely affected generated artifacts.

## Source-grounded conclusions

- Mutable builders/bindings require exclusive construction ownership. Compiled
  metadata and fresh parser state do not synchronize borrowed callbacks, IO or
  custom typed defaults. Exact built-in string slices/maps are copied; generic
  custom graphs are ordinary Go value assignments, not deep immutable snapshots.
- Remainder is positional resolution, not a parser-mode switch; `--` owns literal
  hyphen-prefixed argv. Scalar parsers consume the last token, repeated string
  bindings preserve order, and digit shorthand precedence is command-local.
- Only the selected command’s lifecycle callbacks run; ancestor hooks are not
  inherited. Invocation validation runs before middleware; failure there skips cleanup.
  Lifecycle returned errors/short-circuits still attempt reverse cleanup using
  preserved context values, detached cancellation and one shared 30-second
  cooperative deadline. Returned errors join primary-first; callbacks/IO are
  not preempted, panics are not recovered and no hidden worker is added.
- Continuation CAS prevents duplicate/late `Next` and joins started work before
  cleanup. Its done publication exists only on normal downstream return. A
  middleware recovering a downstream panic can leave the join unfinished. This
  existing runtime limitation is documented and reported to the coordinator,
  not repaired or certified by a new behavior test in this prose assignment.
- CommandSet has no descendants below its direct children, lifecycle hooks or
  completion entry point. Persistent options cannot inherit further and declared
  providers are never called there; normal output/callback obligations remain.
- Output snapshots encode before publication, retaining the last successful data
  on later errors. Reflection preflight covers values and encoder-reachable nil/
  empty-container types, bounded metadata and escape amplification before unsafe
  application serialization callbacks. Buffer caps do not preempt caller writers,
  and Info/success data is not automatically secret-redacted.
- Completion receives hostile partial data; provider allocation, prefix filtering,
  external access and cooperation remain caller-owned. Post-return inspection,
  sanitization, deduplication and count/byte caps protect framework work only.
  Script templates quote token arrays, not evaluated command strings.
- ShutdownController owns cancel-cause state and second-signal channel closure,
  not OS signal registration or exit. Close is idempotent, does not close Forced
  and requires the caller to stop/join its own signal-forwarding work. Repository
  process fixture explicitly owns that goroutine and immediate forced exit.
- Manifest schema is `go-cli/manifest/v1`; output schema is `go-cli/v1`.
  Manifest JSON retains declared metadata, unlike terminal-filtered help/Markdown
  and candidate text. Hidden commands remain in manifest/reference output.
- Native API selection is apidiff `api/baseline.txt`, with independent structural
  `TestPublicAPICompatibility` oracle `api/public.txt`. No native Go-doc snapshot
  is configured. `cmd/generate-reference` writes six artifacts from the fixed
  metadata fixture; generation is non-transactional and requires trusted output.
  Historical benchmark fingerprints/results are not rewritten for prose changes.

## Selected checks

Pinned formatting; all-file executable token/comment-free AST/directive comparison
against base; native rendered public and private package docs; shared source
inventory/check; affected local links/markers/required-document/example structure;
source-grounded reference-generation input review and native structural API
oracle; changed-content secrets and one ordinary complete final-diff review.
No unchanged runtime, race/fuzz/mutation/benchmark/process/shell campaign.
Own three reusable caches for the serialized Go session and clean immediately.
Material consumer-visible clarifications receive an Unreleased entry.
Catalog reverse consumers include Cache, Config, Kafka service adapters, Lease,
Migrations, Postgres, Queue service adapters, Scheduler, Service and its harnesses,
and Telemetry; signatures and executable semantics are preserved for all.

## Results

- Audit covers all 47 tracked Go files, seven package directories, public/internal
  declarations and private lifecycle/parser/output constraints. The source AST
  inventory contains 335 public declarations/members; test-only private capitalized
  fixtures are not mechanically documented for a count.
- Shared `golib source-docs inventory --json`/`check --json`: two modules,
  47 Go files and no findings. No platform/generated-Go variants are skipped;
  compiler/analyzer/security annotations retain exact text.
- `GOTOOLCHAIN=go1.27.2` formatting and disposable stdlib parser/scanner proof:
  all-file code tokens, comment-free AST and directives equal the recorded base.
  Only implicit closing-delimiter semicolons are normalized; explicit tokens,
  callback bodies, tests, fixtures and custom serialization guards are preserved.
- Native `go doc -all` rendered root, clitest, engine, referenceapp and both
  repository commands; stdlib Go-doc parsing rendered the test-only benchmark
  package without importing its comparison dependencies. Changed contract docs
  and representative fields/interfaces were inspected. Final root rendering stable.
- Native structural selector `go test -mod=readonly . -run
  '^(TestPublicAPICompatibility|TestRequiredDocumentationSetIsPresentAndSubstantive)$'
  -count=1` passed. Its initial failure exposed field-layout whitespace in
  `api/public.txt`, not declaration drift. Native `UPDATE_API=1` regeneration
  refreshed that artifact; whitespace-normalized base/current content is identical,
  a second regeneration is byte-stable, and normal API validation passed again.
  Configured apidiff `api/baseline.txt` remains unchanged; no full Go-doc baseline
  or unrelated release artifact is introduced.
- Native `go run ./cmd/generate-reference -out OWNED_TEMP` plus `diff -ru` against
  `docs/generated` reproduced all six artifacts byte-for-byte. No changed script,
  live-shell or terminal behavior needs a campaign; historical benchmark outputs
  and provenance stay untouched.
- Local Markdown links/anchors, unchanged example structure/output ownership,
  marker/generator inventory, staged-content secrets and ordinary complete final-
  diff review passed. Intent/ACK profile/message validation and normal commit hooks
  precede coordinator delivery.
- No runtime/lifecycle regression, race/leak, active-fuzz, mutation, benchmark,
  subprocess/live-shell/browser, reverse-consumer or release campaign was run.
  Previously useful unchanged runtime evidence is not relabeled as fresh proof.
  Exact-source hosted CI and main publication remain coordinator-owned.
- Serialized native session reused three task-owned caches. Build/module/temp
  caches, proof helper, package renderings and generated artifact copies were
  removed; the committed worktree is preserved for delivery.

No unresolved rationale prevents source documentation. The source-observed
recovered-Next-panic continuation limitation remains an existing runtime issue,
reported to the coordinator and explicitly outside this comment-only repair;
no panic-recovery fix or new behavioral proof is claimed.

## Objective Enforcement

The qualified public Tools v2.4.0 binary and immutable reusable CI source
provide the strict tracked-source check through `make source-docs-check`,
the existing local check/CI routes and Required aggregation. Release and
selected security/assurance routes retain their existing controls. This
structural status supplements the rendered semantic audit; it does not
certify prose meaning, generator execution or runtime conformance.
