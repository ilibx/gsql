BIN := bin/gsql
SQL_FILE ?= samples/query_check.sql
OUT_FILE := $(shell mktemp -t gsql_query_out.XXXXXX)

.PHONY: all build run-sql check-sql test clean

all: build

build:
	@mkdir -p bin
	go build -o $(BIN) ./cmd/gsql

run-sql: build
	@echo "Running SQL from $(SQL_FILE)"
	@$(BIN) $(SQL_FILE)

check-sql: build
	@echo "Running SQL from $(SQL_FILE) and checking output"
	@mkdir -p bin
	@$(BIN) $(SQL_FILE) | grep -E '^(id, name|[0-9]+, )' > $(OUT_FILE)
	@diff -u samples/query_check.expected $(OUT_FILE)
	@echo "SQL execution result matches expected output."

test:
	go test ./...

clean:
	rm -rf bin
