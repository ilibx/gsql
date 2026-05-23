package main

import (
    "fmt"
    "os"

    "github.com/ilibx/gsql/pkg/catalog"
    "github.com/ilibx/gsql/pkg/engine"
    "github.com/ilibx/gsql/pkg/parser"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: gsql [desc] -s <sql-file> [-e <sql-statement>] ...")
        os.Exit(1)
    }

    args := os.Args[1:]
    verbose := false

    if args[0] == "desc" {
        verbose = true
        args = args[1:]
    }

    if len(args) == 0 {
        fmt.Fprintln(os.Stderr, "error: expected -s or -e arguments")
        os.Exit(1)
    }

    cat := catalog.NewCatalog()
    eng := engine.NewEngine(cat)
    eng.Verbose = verbose
    p := parser.NewParser()

    for i := 0; i < len(args); i++ {
        switch args[i] {
        case "-s":
            i++
            if i >= len(args) {
                fmt.Fprintln(os.Stderr, "error: expected sql file after -s")
                os.Exit(1)
            }
            data, err := os.ReadFile(args[i])
            if err != nil {
                fmt.Fprintf(os.Stderr, "read sql file failed: %v\n", err)
                os.Exit(1)
            }
            stmts, err := p.Parse(string(data))
            if err != nil {
                fmt.Fprintf(os.Stderr, "parse sql failed: %v\n", err)
                os.Exit(1)
            }
            for _, stmt := range stmts {
                if err := eng.Execute(stmt); err != nil {
                    fmt.Fprintf(os.Stderr, "execution failed: %v\n", err)
                    os.Exit(1)
                }
            }
        case "-e":
            i++
            if i >= len(args) {
                fmt.Fprintln(os.Stderr, "error: expected sql statement after -e")
                os.Exit(1)
            }
            stmts, err := p.Parse(args[i])
            if err != nil {
                fmt.Fprintf(os.Stderr, "parse sql failed: %v\n", err)
                os.Exit(1)
            }
            for _, stmt := range stmts {
                if err := eng.Execute(stmt); err != nil {
                    fmt.Fprintf(os.Stderr, "execution failed: %v\n", err)
                    os.Exit(1)
                }
            }
        default:
            fmt.Fprintf(os.Stderr, "error: unexpected argument %s (expected -s or -e)\n", args[i])
            os.Exit(1)
        }
    }
}
