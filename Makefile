BIN := bin/gsql
SQL_FILE ?= tests/check.sql
OUT_FILE := $(shell mktemp -t gsql_query_out.XXXXXX)

.PHONY: all build run-sql check-sql test clean

all: build

build:
	@mkdir -p bin
	go build -o $(BIN) ./cmd/gsql

run-sql: build
	@echo "Running SQL from $(SQL_FILE)"
	@$(BIN) -s $(SQL_FILE)

check-sql: build
	@echo "Running SQL from $(SQL_FILE) and checking output"
	@mkdir -p bin
	@$(BIN) -s $(SQL_FILE) | grep -E '^(id, name|[0-9]+, )' > $(OUT_FILE)
	@diff -u tests/check.expected $(OUT_FILE)
	@echo "SQL execution result matches expected output."

test:
	go test ./...

clean:
	rm -rf bin
