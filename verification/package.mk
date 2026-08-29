.PHONY: benchmark docs

benchmark:
	GOWORK=off go test . -run '^$$' -bench Benchmark -benchmem -benchtime=100ms
	cd benchmarks && GOWORK=$$(cd .. && pwd)/go.work go test ./... -bench 'BenchmarkEquivalent(Construction|Dispatch)$$' -benchmem -benchtime=100ms

docs:
	./scripts/check-generated.sh
	GOWORK=off go test . -run 'TestRequiredDocumentation|Example' -count=1
