# AIrtg schema - builds and checks.
#
#   make              every check (same as `make check`)
#   make generate     regenerate the Go code of airtg/ from proto/
#   make fixtures     write the files of fixtures added to fixtures/fixtures_test.go
#   make check        formats, lint, generated code up to date, go.mod tidy, vet, tests
#   make breaking     compare with the last release: wire/JSON compatibility, fixtures unchanged
#
# Tools are pinned Go tools, nothing to install: buf in tools/go.mod (its own
# module, so its dependencies never reach consumers), protoc-gen-go in go.mod
# (the same version as the protobuf runtime consumers get).

GO   ?= go
BUF  := $(GO) tool -modfile=tools/go.mod buf

.DEFAULT_GOAL := check
.PHONY: check generate fixtures fmt-check vet test lint generated-check tidy-check breaking help

check: fmt-check lint generated-check tidy-check vet test ## Every check

generate: ## Regenerate airtg/ from proto/
	$(BUF) generate

fixtures: ## Write the files of new fixtures (never rewrites existing ones)
	$(GO) test ./fixtures -run '^TestUpdate$$' -update -v

fmt-check:
	@files=$$(gofmt -l .); if [ -n "$$files" ]; then echo "not gofmt'ed:"; echo "$$files"; exit 1; fi
	$(BUF) format --diff --exit-code

lint: ## buf lint: naming, layout, every element documented
	$(BUF) lint

# Generates into .gen-check/ (git-ignored; buf's clean: true empties
# .gen-check/airtg first) and compares, so it works whatever the git state.
generated-check: ## The committed Go code matches proto/
	$(BUF) generate -o .gen-check
	@diff -r airtg .gen-check/airtg || { echo "airtg/ is not up to date: run make generate"; exit 1; }

tidy-check: ## go.mod and go.sum of both modules are tidy
	$(GO) mod tidy -diff
	cd tools && $(GO) mod tidy -diff

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

# Compared with the last release tag (v*). From 1.0.0 a difference fails;
# before, while the schema may still change freely, it is only reported.
# Needs the git history and tags (CI: fetch-depth 0).
breaking: ## Compare with the last release (fails from 1.0.0, warns before)
	@set -e; \
	tag=$$(git tag --list 'v*' --sort=-v:refname | head -n 1); \
	if [ -z "$$tag" ]; then echo "no release yet: nothing to compare with"; exit 0; fi; \
	major=$$(cut -d. -f1 VERSION); failed=0; \
	echo "comparing with $$tag"; \
	$(BUF) breaking --against ".git#tag=$$tag" || failed=1; \
	changed=$$(git diff --name-only --diff-filter=DMRT "$$tag" -- fixtures/v1); \
	if [ -n "$$changed" ]; then echo "fixtures changed or removed since $$tag (they are append-only):"; echo "$$changed"; failed=1; fi; \
	if [ $$failed = 1 ]; then \
		if [ "$$major" = 0 ]; then echo "WARNING: incompatible with $$tag (allowed before 1.0.0)"; \
		else echo "incompatible with $$tag: add a new package (airtg/v2) instead"; exit 1; fi; \
	fi

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'
