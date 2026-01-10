all: build

.PHONY: build
build:
	@echo "Building fxrates binary"
	go build -o build/fxrates ./cmd

.PHONY: lint
lint:
	golangci-lint run --config .github/golangci.yaml

.PHONY: gofumpt
gofumpt:
	go install mvdan.cc/gofumpt@latest
	gofumpt -l -w .

.PHONY: fixalign
fixalign:
	go install golang.org/x/tools/go/analysis/passes/fieldalignment/cmd/fieldalignment@latest
	fieldalignment -fix $(filter-out $@,$(MAKECMDGOALS)) # the full package name (not path!)

.PHONY: graphqlgen
graphqlgen:
	go run github.com/99designs/gqlgen generate