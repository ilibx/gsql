package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/flosch/pongo2/v5"
	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

// parseParamValue parses a parameter value.
//   - If it starts with '@', reads the file and parses as JSON.
//   - If it looks like JSON ([...] or {...}), parses as JSON (preserves integer types).
//   - Otherwise returns the raw string.
func parseParamValue(raw string) interface{} {
	raw = strings.TrimSpace(raw)

	var candidate string
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimPrefix(raw, "@")
		data, err := os.ReadFile(path)
		if err != nil {
			return raw
		}
		candidate = strings.TrimSpace(string(data))
	} else {
		candidate = raw
	}

	if len(candidate) > 0 && (candidate[0] == '[' || candidate[0] == '{') {
		var val interface{}
		dec := json.NewDecoder(strings.NewReader(candidate))
		dec.UseNumber()
		if err := dec.Decode(&val); err == nil {
			return jsonCleanNumbers(val)
		}
	}

	if strings.HasPrefix(raw, "@") {
		return candidate
	}
	return raw
}

// jsonCleanNumbers converts json.Number to int64 or float64 as appropriate.
func jsonCleanNumbers(v interface{}) interface{} {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, _ := x.Float64()
		return f
	case map[string]interface{}:
		for k, val := range x {
			x[k] = jsonCleanNumbers(val)
		}
	case []interface{}:
		for i, val := range x {
			x[i] = jsonCleanNumbers(val)
		}
	}
	return v
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("  gsql [desc] [-v]... -s <sql-file|url> [-e <sql-statement>] ...")
		fmt.Println("  gsql [desc] [-v]... -t <template-file|url> -a <key=value> ...")
		fmt.Println("")
		fmt.Println("Flags:")
		fmt.Println("  -s   Execute SQL from file/URL")
		fmt.Println("  -e   Execute inline SQL")
		fmt.Println("  -t   Render Jinja2 template from file/URL, then execute")
		fmt.Println("  -a   Template parameter (key=value), repeatable")
		fmt.Println("       JSON values: key=[...] or key={...} for arrays/objects")
		fmt.Println("       File values:  key=@file.json reads file as JSON")
		fmt.Println("  -v   Verbose/debug mode")
		fmt.Println("")
		fmt.Println("URL schemes: s3://, lark://, ftp://, sftp://, http://, https://, webdav://, gitlfs://, local://")
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

	// Second pass: collect -a parameters (key=value pairs)
	params := make(map[string]interface{})
	filtered = nil
	for i := 0; i < len(args); i++ {
		if args[i] == "-a" {
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "error: expected parameter after -a")
				os.Exit(1)
			}
			arg := args[i]
			eqIdx := strings.IndexByte(arg, '=')
			if eqIdx < 0 {
				fmt.Fprintf(os.Stderr, "error: invalid parameter format: %s (expected key=value)\n", arg)
				os.Exit(1)
			}
			key := strings.TrimSpace(arg[:eqIdx])
			val := strings.TrimSpace(arg[eqIdx+1:])
			if key == "" {
				fmt.Fprintf(os.Stderr, "error: empty key in parameter: %s\n", arg)
				os.Exit(1)
			}
			params[key] = parseParamValue(val)
		} else {
			filtered = append(filtered, args[i])
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
		case "-t":
			i++
			if i >= len(args) {
				fmt.Fprintln(os.Stderr, "error: expected template file after -t")
				os.Exit(1)
			}
			arg := args[i]

			// read template content
			var data []byte
			var err error
			if strings.HasPrefix(arg, "local://") {
				data, err = os.ReadFile(strings.TrimPrefix(arg, "local://"))
			} else if storage.IsURLScheme(arg) {
				data, err = storage.ReadFromURL(context.Background(), arg)
			} else {
				data, err = os.ReadFile(arg)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "read template file failed: %v\n", err)
				os.Exit(1)
			}

			// render template with Jinja2 syntax via pongo2
			tpl, err := pongo2.FromString(string(data))
			if err != nil {
				fmt.Fprintf(os.Stderr, "parse template failed: %v\n", err)
				os.Exit(1)
			}

			ctx := pongo2.Context{}
			for k, v := range params {
				ctx[k] = v
			}

			rendered, err := tpl.Execute(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr, "render template failed: %v\n", err)
				os.Exit(1)
			}

			if debugLevel > 0 {
				fmt.Fprintf(os.Stderr, "--- rendered SQL ---\n%s\n---\n", rendered)
			}

			stmts, err := p.Parse(rendered)
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
			fmt.Fprintf(os.Stderr, "error: unexpected argument %s (expected -s, -e, or -t)\n", args[i])
			os.Exit(1)
		}
	}
}
