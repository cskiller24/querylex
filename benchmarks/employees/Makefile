.PHONY: setup up down clean

DATA_DIR := $(PWD)/data
TEST_DB_REPO := https://github.com/datacharmer/test_db.git
TEST_DB_TAG := v1.0.7

setup:
	@echo "Downloading employee sample database..."
	@if [ -d "$(DATA_DIR)/test_db" ]; then \
		echo "test_db already exists, updating..."; \
		cd "$(DATA_DIR)/test_db" && git pull; \
	else \
		mkdir -p "$(DATA_DIR)"; \
		git clone --depth 1 --branch $(TEST_DB_TAG) $(TEST_DB_REPO) "$(DATA_DIR)/test_db"; \
	fi
	@echo "Employee sample database ready at $(DATA_DIR)/test_db"

up:
	docker compose up -d
	@echo ""
	@echo "Databases starting..."
	@echo "  MySQL 8.0:    localhost:3306 (user: employees, pass: employees, db: employees)"
	@echo "  PostgreSQL 16: localhost:5432 (user: employees, pass: employees, db: employees)"
	@echo ""
	@echo "Wait ~30s for initialization, then check with:"
	@echo "  docker compose logs -f"

down:
	docker compose down

clean:
	docker compose down -v
	rm -rf $(DATA_DIR)
