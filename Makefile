.PHONY: build test clean install lint release completions

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")
LDFLAGS := -s -w \
	-X github.com/cskiller24/querylex/internal/version.Version=$(VERSION) \
	-X github.com/cskiller24/querylex/internal/version.Commit=$(COMMIT) \
	-X github.com/cskiller24/querylex/internal/version.BuildDate=$(DATE)

build:
	go build -ldflags="$(LDFLAGS)" -o bin/querylex ./cmd/querylex/
	go build -ldflags="$(LDFLAGS)" -o bin/querylex-add-db ./cmd/querylex-add-db/
	go build -ldflags="$(LDFLAGS)" -o bin/querylex-stats ./cmd/querylex-stats/

test:
	go test ./... -short -count=1

clean:
	rm -rf bin/

install:
	go install -ldflags="$(LDFLAGS)" ./cmd/querylex/
	go install -ldflags="$(LDFLAGS)" ./cmd/querylex-add-db/
	go install -ldflags="$(LDFLAGS)" ./cmd/querylex-stats/

lint:
	go vet ./...

completions:
	go run ./cmd/generate_completions/

# GOMAXPROCS limits Go compiler parallelism; --parallelism limits goreleaser task
# concurrency. Both default to #CPUs which can OOM on low-memory systems (2GB).
# Override via: make release PARALLELISM=4
PARALLELISM ?= 2
release:
	goreleaser release --clean --parallelism=$(PARALLELISM)
