COVERAGE_FILE ?= coverage.out
HTML_COVERAGE ?= coverage.html

# Get all directories in cmd/ as available modules
MODULES := $(notdir $(wildcard cmd/*))

# Help target - display usage information
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  \033[36mmake build\033[0m - Build all modules ($(MODULES))"
	@$(foreach mod,$(MODULES),echo "  \033[36mmake build_$(mod)\033[0m - Build $(mod) module";)
	@echo "  \033[36mmake test\033[0m - Run fast tests (integration excluded)"
	@echo "  \033[36mmake test-slow\033[0m - Run slow integration tests"
	@echo "  \033[36mmake test-all\033[0m - Run all kind of tests"
	@echo "  \033[36mmake html_test\033[0m - Generate html test report"
	@echo "  \033[36mmake fmt\033[0m - Run go fmt for all project"
	@echo "  \033[36mmake lint\033[0m - Run golangci-lint for all project"
	@echo "  \033[36mmake clean\033[0m - Clean test artefacts"
	@echo "  \033[36mmake run\033[0m - Run selected service(exm: make run SERVICE=<name_service>)"
	@echo "  \033[36mmake migrate_up\033[0m - Apply all migrations"
	@echo "  \033[36mmake migrate_down\033[0m - Rollback 1 migration"

## build: build all services
.PHONY: build
build:
	@echo "Building all modules: $(MODULES)"
	@mkdir -p bin
	@$(foreach mod,$(MODULES),echo "Building module: $(mod)"; go build -o ./bin/$(mod) ./cmd/$(mod);)

# Convenience targets for building individual modules
.PHONY: $(addprefix build_,$(MODULES))
$(addprefix build_,$(MODULES)):
	@modulename=$(subst build_,,$@); \
	echo "Building module: $$modulename"; \
	mkdir -p bin; \
	go build -o ./bin/$$modulename ./cmd/$$modulename

## run: run specific service(make run SERVICE=<name_service>
.PHONY: run
run:
	@$(MAKE) build
	@./bin/$(SERVICE) 

## test: run fast unit tests
.PHONY: test
test:
	@echo "Running fast tests..."
	@go test -coverpkg='github.com/n1jke/linktracker/...' --race -count=1 -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

## test-slow: run slow integration tests
.PHONY: test-slow 
test-slow:
	@echo "Running integration tests..."
	@go test -v -tags=integration -race -count=1 -timeout=90s ./internal/tests/... -v

## test-all: run all kind of tests
.PHONY: test-slow 
test-all:
	@echo "Running all tests..."
	@go test -coverpkg='github.com/n1jke/linktracker/...' --race -count=1 -tags=integration -timeout=90s -coverprofile='$(COVERAGE_FILE)' ./...
	@go tool cover -func='$(COVERAGE_FILE)' | grep ^total | tr -s '\t'

## html_test: generate html test report
.PHONY: html_test
html_test:
	@go tool cover -html='$(COVERAGE_FILE)' -o $(HTML_COVERAGE)
	@echo "Coverage report saved to $(HTML_COVERAGE)"

# fmt: run go fmt for all project
.PHONY: fmt
fmt:
	@echo "go fmt ./..."
	@go fmt ./...

## lint: run golangci-lint for all project
.PHONY: lint
lint:
	@golangci-lint --version && echo "golangci-lint -v run --fix ./..." || echo "golangci-lint not found"
	@golangci-lint -v run --fix ./...

# clean: clean all artefacts
.PHONY: clean
clean:
	@echo "Cleaning test artefacts..."
	@rm -f $(COVERAGE_FILE)
	@rm -f $(HTML_COVERAGE)

.PHONY: migrate_up 
migrate_up:
	@migrate -path ./migrations -database "$(DB_URL)" up

.PHONY: migrate_down 
migrate_down:
	@migrate -path  ./migrations -database "$(DB_URL)" down 
