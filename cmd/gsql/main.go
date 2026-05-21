package main

import (
    "fmt"
    "os"

    "github.com/ilibx/gsql/pkg/catalog"
    "github.com/ilibx/gsql/pkg/engine"
    "github.com/ilibx/gsql/pkg/sqlparse"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: gsql <sql-file>")
        os.Exit(1)
    }

    path := os.Args[1]
    content, err := os.ReadFile(path)
    if err != nil {
        fmt.Fprintf(os.Stderr, "read sql file failed: %v\n", err)
        os.Exit(1)
    }

    parser := sqlparse.NewParser()
    statements, err := parser.Parse(string(content))
    if err != nil {
        fmt.Fprintf(os.Stderr, "parse sql failed: %v\n", err)
        os.Exit(1)
    }

    catalog := catalog.NewCatalog()
    engine := engine.NewEngine(catalog)
    for _, stmt := range statements {
        if err := engine.Execute(stmt); err != nil {
            fmt.Fprintf(os.Stderr, "execution failed: %v\n", err)
            os.Exit(1)
        }
    }

    fmt.Println("Execution completed.")
}
