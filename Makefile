.PHONY: build test clean install lint

build:
	go build -o bin/querylex ./cmd/querylex/
	go build -o bin/querylex-add-db ./cmd/querylex-add-db/
	go build -o bin/querylex-stats ./cmd/querylex-stats/

test:
	go test ./... -short -count=1

clean:
	rm -rf bin/

install:
	go install ./cmd/querylex/
	go install ./cmd/querylex-add-db/
	go install ./cmd/querylex-stats/

lint:
	go vet ./...
