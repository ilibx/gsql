package main

import (
	"fmt"
	"github.com/ilibx/gsql/pkg/sqlparse"
)

func main() {
	sqls := []string{
		"SELECT name, age + 1 FROM users;",
		"SELECT name, age + 1 AS next_age FROM users;",
		"SELECT name, age + 1 AS next_age, email FROM users;",
	}
	
	for i, sql := range sqls {
		fmt.Printf("\nTest %d: %s\n", i+1, sql)
		parser := sqlparse.NewParser()
		stmts, err := parser.Parse(sql)
		if err != nil {
			fmt.Printf("  ERROR: %v\n", err)
		} else {
			fmt.Printf("  SUCCESS: %d statements\n", len(stmts))
		}
	}
}
