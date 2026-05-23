package main

import (
	"fmt"
	"github.com/ilibx/gsql/pkg/sqlparse"
)

func main() {
	sql := "SELECT name, age + 1 AS next_age, email FROM users WHERE age + 5 > 25 ORDER BY next_age LIMIT 5;"
	fmt.Printf("Test: %s\n", sql[:min(60, len(sql))])
	parser := sqlparse.NewParser()
	stmts, err := parser.Parse(sql)
	if err != nil {
		fmt.Printf("  ERROR: %v\n", err)
	} else {
		fmt.Printf("  SUCCESS: %d statements\n", len(stmts))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
