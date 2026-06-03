.PHONY: build test clean install lint release completions compose-up-mysql compose-up-postgresql compose-up-mariadb compose-up-mssql compose-down build-test test-e2e-mysql test-e2e-postgresql test-e2e-mariadb test-e2e-mssql

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")
LDFLAGS := -s -w \
	-X github.com/cskiller24/querylex/internal/version.Version=$(VERSION) \
	-X github.com/cskiller24/querylex/internal/version.Commit=$(COMMIT) \
	-X github.com/cskiller24/querylex/internal/version.BuildDate=$(DATE)

build:
	go build -ldflags="$(LDFLAGS)" -o bin/querylex ./cmd/querylex/

test:
	go test ./... -short -count=1

clean:
	rm -rf bin/

install:
	go install -ldflags="$(LDFLAGS)" ./cmd/querylex/

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

# Docker Compose targets — start one database engine at a time.
# Each target uses a Compose profile so only the requested engine starts.
# Ports are random (compose.yaml ports: ["0"]) — resolved at runtime.

compose-up-mysql:
	docker compose --profile mysql up -d --wait

compose-up-postgresql:
	docker compose --profile postgresql up -d --wait

compose-up-mariadb:
	docker compose --profile mariadb up -d --wait

compose-up-mssql:
	docker compose --profile mssql up -d --wait

# Stops and removes all database containers and their tmpfs volumes.
compose-down:
	docker compose --profile mysql --profile postgresql --profile mariadb --profile mssql down --volumes 2>/dev/null; \
	docker compose down --volumes 2>/dev/null; \
	true

# Build the E2E test binary with the e2e build tag.
# Output: bin/e2e.test — a compiled test binary that can be run directly.
build-test: build
	go test -tags e2e -c -o bin/e2e.test ./test/mysql/

# E2E test targets — full cycle: start DB, run tests, stop DB.
# Each target starts the specific database profile, waits for health,
# runs the compiled E2E test binary, then stops the container.
# Exit code is captured so compose-down runs even on test failure.

test-e2e-mysql: build-test compose-up-mysql
	TEST_MYSQL_DSN="$$(./test/scripts/resolve-port.sh mysql 3306)" \
	./bin/e2e.test -test.run "TestMySQL" -test.v; \
	EXIT_CODE=$$?; \
	$(MAKE) compose-down; \
	exit $$EXIT_CODE

test-e2e-postgresql: build-test compose-up-postgresql
	TEST_PG_DSN="$$(./test/scripts/resolve-port.sh postgresql 5432)" \
	./bin/e2e.test -test.run "TestPostgreSQL" -test.v; \
	EXIT_CODE=$$?; \
	$(MAKE) compose-down; \
	exit $$EXIT_CODE

test-e2e-mariadb: build-test compose-up-mariadb
	TEST_MARIADB_DSN="$$(./test/scripts/resolve-port.sh mariadb 3306)" \
	./bin/e2e.test -test.run "TestMariaDB" -test.v; \
	EXIT_CODE=$$?; \
	$(MAKE) compose-down; \
	exit $$EXIT_CODE

test-e2e-mssql: build-test compose-up-mssql
	TEST_MSSQL_DSN="$$(./test/scripts/resolve-port.sh mssql 1433)" \
	./bin/e2e.test -test.run "TestMSSQL" -test.v; \
	EXIT_CODE=$$?; \
	$(MAKE) compose-down; \
	exit $$EXIT_CODE
