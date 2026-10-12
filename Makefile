GOLIB ?= golib

.PHONY: source-docs-check check check-full ci cohesion inventory repository-check specification-check specification-check-online

check: source-docs-check
	$(GOLIB) check --local --all

check-full: source-docs-check
	$(GOLIB) check --all

ci: source-docs-check repository-check cohesion specification-check check

cohesion:
	$(GOLIB) cohesion check

inventory repository-check:
	$(GOLIB) repository check

specification-check:
	$(GOLIB) specification check

specification-check-online:
	$(GOLIB) specification check --online

source-docs-check:
	$(GOLIB) source-docs check --json
