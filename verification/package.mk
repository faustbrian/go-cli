.PHONY: benchmark docs

benchmark:
	GOWORK=off go test . -run '^$$' -bench Benchmark -benchmem -benchtime=100ms
	GOWORK=$$(pwd)/go.work go test ./benchmarks/... -run '^(TestComparisonOutputMatchesOwnedBoundsAndWriterContract|TestEquivalentBenchmarkRunnersIsolateRepeatedState|TestEquivalentBenchmarkRunnersShareObservableContract|TestEquivalentBenchmarkRunnersShareStructuralFailure|TestOwnedParserMatchesTheFormerCobraAdapter)$$' -count=1
	GOWORK=$$(pwd)/go.work go test ./benchmarks/... -run '^$$' -bench 'BenchmarkEquivalent(Cold|Repeated)Invocation$$|BenchmarkParsingFloorFlag$$' -benchmem -benchtime=100ms

docs:
	./scripts/check-generated.sh
	GOWORK=off go test . -run 'TestRequiredDocumentation|Example' -count=1
