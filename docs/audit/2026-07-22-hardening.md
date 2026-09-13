# Hardening audit: 2026-07-22

This report records the evidence-driven hardening review of `go-cli`. The
review covers the command graph, parser boundary, lifecycle, generated
artifacts, dependency boundary, documentation, and release gates. It remains
historical evidence for the current public contract and is kept alongside the
executable tests that enforce those claims.

## Command graph conformance

The audit exercised empty, broad, deep, and maximum command graphs; aliases,
options, arguments, required groups, cycles, reused nodes, hidden and
deprecated metadata, stable ordering, concurrent reads, and hostile graph
limits. Invalid graphs are rejected before handlers run, and valid graphs are
immutable after construction.

## Lifecycle and failure matrix

Construction, dispatch, cancellation, middleware, non-interactive execution,
shutdown, signal handling, and late continuation failures were checked as a
single lifecycle. The matrix records success, usage, cancellation, panic, and
writer-failure outcomes, including cleanup and error ownership at each phase.

## Mutation and release evidence

The historical mutation run killed 674 killed mutants, with zero survivors and
zero timeouts among covered production mutants. The remaining not-covered
cases are listed in the mutation report and are not silently promoted to
passes.

## Findings registry

The findings registry records command graph ambiguity, completion safety,
secret metadata, parser diagnostics, lifecycle cancellation, output
isolation, shutdown release, and aggregate-bound enforcement. Each finding has
an owning maintainer, a regression assertion, and a documented disposition.

## Release gate inventory

The release gate inventory binds formatting, build, vet, unit and subprocess
tests, race and concurrency checks, generated documentation, API and
architecture checks, benchmarks, mutation evidence, dependency and secret
scans, SBOM, and reproducible source archives. The current gate remains
executable through the repository's documented commands.
