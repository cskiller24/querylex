.PHONY: generate up down clean

SCALE_FACTOR ?= 1
DATA_DIR := $(PWD)/data
GENERATOR_IMAGE := tpch-generator:latest

generate:
	@echo "Building TPC-H data generator..."
	docker build -t $(GENERATOR_IMAGE) -f Dockerfile.generator .
	@echo "Generating TPC-H data (SF=$(SCALE_FACTOR), ~$(SCALE_FACTOR)GB)..."
	mkdir -p $(DATA_DIR)
	docker run --rm \
		-v $(DATA_DIR):/data \
		-e SCALE_FACTOR=$(SCALE_FACTOR) \
		$(GENERATOR_IMAGE)
	@echo ""
	@echo "Data generated at $(DATA_DIR)/"
	@ls -lh $(DATA_DIR)

up:
	docker compose up -d
	@echo ""
	@echo "TPC-H databases starting (SF=$(SCALE_FACTOR))..."
	@echo "  MySQL 8.0:    localhost:3307 (user: tpch, pass: tpch, db: tpch)"
	@echo "  PostgreSQL 16: localhost:5433 (user: tpch, pass: tpch, db: tpch)"
	@echo ""
	@echo "MySQL data loading may take several minutes for SF >= 1."
	@echo "Monitor progress: docker compose logs -f mysql"
	@echo "PostgreSQL loads concurrently."

down:
	docker compose down

clean:
	docker compose down -v
	rm -rf $(DATA_DIR)
