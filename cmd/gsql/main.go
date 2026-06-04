package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

func main() {
    if len(os.Args) < 2 {
		fmt.Println("Usage: gsql [desc] [-v]... -s <sql-file|url> [-e <sql-statement>] ...")
		fmt.Println("  -s supports local files and storage URLs (s3://, lark://, ftp://, sftp://, http://, https://, webdav://, gitlfs://)")
        os.Exit(1)
    }

    args := os.Args[1:]
    debugLevel := 0

    // First pass: extract desc and all -v/-vvvv flags
    var filtered []string
    for _, a := range args {
        switch {
        case a == "desc":
            if debugLevel == 0 {
                debugLevel = 1
            }
        case a == "-v":
            debugLevel++
        case strings.HasPrefix(a, "-v") && len(a) > 2:
            debugLevel += len(a) - 1 // -vv → 2, -vvv → 3, etc.
        default:
            filtered = append(filtered, a)
        }
    }
    args = filtered

    cat := catalog.NewCatalog()
    eng := engine.NewEngine(cat)
    eng.VerboseLevel = debugLevel
    catalog.DebugLevel = debugLevel
    p := parser.NewParser()

    for i := 0; i < len(args); i++ {
        switch args[i] {
		case "-s":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "error: expected sql file after -s")
				os.Exit(1)
			}
			arg := args[i]
			var data []byte
			var err error
			if storage.IsURLScheme(arg) {
				data, err = storage.ReadFromURL(context.Background(), arg)
			} else {
				data, err = os.ReadFile(arg)
			}
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
