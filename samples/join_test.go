package samples

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/engine"
	"github.com/ilibx/gsql/pkg/parser"
	"github.com/ilibx/gsql/pkg/storage"
)

// joinTestSchema creates customers (id 1..5) and orders (customer_id
// 1,1,2,9,3) so that every join type has a hand-checkable result.
//
//	matched customer ids: {1,2,3}
//	orders with no customer: 104 (customer_id=9)
//	customers with no orders: 4, 5
func joinTestSchema(t *testing.T) *engine.Engine {
	t.Helper()
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	p := parser.NewParser()

	stmts, err := p.Parse(`CREATE TABLE customers (
		id INT,
		name STRING,
		city STRING
	) WITH (
		storage = 'local', format = 'csv',
		path = 'samples/join', file_pattern = 'customers.csv'
	);
	CREATE TABLE orders (
		order_id INT,
		customer_id INT,
		amount INT
	) WITH (
		storage = 'local', format = 'csv',
		path = 'samples/join', file_pattern = 'orders.csv'
	);`)
	if err != nil {
		t.Fatalf("parse schema failed: %v", err)
	}
	for _, stmt := range stmts {
		if err := eng.Execute(stmt); err != nil {
			t.Fatalf("execute schema failed: %v", err)
		}
	}
	return eng
}

// query runs a SELECT and returns the rows.
func queryJoin(t *testing.T, eng *engine.Engine, sql string) []storage.Row {
	t.Helper()
	p := parser.NewParser()
	stmts, err := p.Parse(sql)
	if err != nil {
		t.Fatalf("parse failed: %v\nsql: %s", err, sql)
	}
	for _, stmt := range stmts {
		if s, ok := stmt.(*parser.SelectStmt); ok {
			rows, err := eng.ExecuteSelect(s.Query)
			if err != nil {
				t.Fatalf("select failed: %v\nsql: %s", err, sql)
			}
			return rows
		}
	}
	t.Fatalf("no SELECT in: %s", sql)
	return nil
}

// rowKey renders a row as a stable, comparable string.
func rowKey(row storage.Row, cols ...string) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = c + "=" + row[c]
	}
	return strings.Join(parts, ",")
}

// sortedKeys renders rows and sorts them so comparisons are order-agnostic.
func sortedKeys(rows []storage.Row, cols ...string) []string {
	keys := make([]string, 0, len(rows))
	for _, r := range rows {
		keys = append(keys, rowKey(r, cols...))
	}
	sort.Strings(keys)
	return keys
}

func TestJoinAllTypes(t *testing.T) {
	eng := joinTestSchema(t)

	cases := []struct {
		name string
		sql  string
		want []string
	}{
		// INNER: only matched pairs. customer 1 has 2 orders (101,105? no: 101,102),
		// 2 has 103, 3 has 105.
		{
			name: "inner",
			sql:  "SELECT c.name, o.order_id, o.amount FROM customers c JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"amount=500,name=Alice,order_id=101",
				"amount=150,name=Alice,order_id=102",
				"amount=300,name=Bob,order_id=103",
				"amount=250,name=Charlie,order_id=105",
			},
		},
		// LEFT: all customers; unmatched customers keep empty order fields.
		{
			name: "left",
			sql:  "SELECT c.name, o.order_id FROM customers c LEFT JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"name=Alice,order_id=101",
				"name=Alice,order_id=102",
				"name=Bob,order_id=103",
				"name=Charlie,order_id=105",
				"name=Diana,order_id=0",
				"name=Eve,order_id=0",
			},
		},
		// LEFT OUTER: identical to LEFT.
		{
			name: "left_outer",
			sql:  "SELECT c.name, o.order_id FROM customers c LEFT OUTER JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"name=Alice,order_id=101",
				"name=Alice,order_id=102",
				"name=Bob,order_id=103",
				"name=Charlie,order_id=105",
				"name=Diana,order_id=0",
				"name=Eve,order_id=0",
			},
		},
		// RIGHT: all orders; order 104 (customer_id=9) keeps empty customer.
		{
			name: "right",
			sql:  "SELECT c.name, o.order_id FROM customers c RIGHT JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"name=Alice,order_id=101",
				"name=Alice,order_id=102",
				"name=Bob,order_id=103",
				"name=Charlie,order_id=105",
				"name=,order_id=104",
			},
		},
		// RIGHT OUTER: identical to RIGHT.
		{
			name: "right_outer",
			sql:  "SELECT c.name, o.order_id FROM customers c RIGHT OUTER JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"name=Alice,order_id=101",
				"name=Alice,order_id=102",
				"name=Bob,order_id=103",
				"name=Charlie,order_id=105",
				"name=,order_id=104",
			},
		},
		// FULL: matched pairs + Diana/Eve (no orders) + order 104 (no customer).
		{
			name: "full",
			sql:  "SELECT c.name, o.order_id FROM customers c FULL JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"name=Alice,order_id=101",
				"name=Alice,order_id=102",
				"name=Bob,order_id=103",
				"name=Charlie,order_id=105",
				"name=Diana,order_id=0",
				"name=Eve,order_id=0",
				"name=,order_id=104",
			},
		},
		// FULL OUTER: identical to FULL.
		{
			name: "full_outer",
			sql:  "SELECT c.name, o.order_id FROM customers c FULL OUTER JOIN orders o ON o.customer_id = c.id;",
			want: []string{
				"name=Alice,order_id=101",
				"name=Alice,order_id=102",
				"name=Bob,order_id=103",
				"name=Charlie,order_id=105",
				"name=Diana,order_id=0",
				"name=Eve,order_id=0",
				"name=,order_id=104",
			},
		},
		// LEFT SEMI: only customers that have at least one order (each once).
		{
			name: "left_semi",
			sql:  "SELECT c.name FROM customers c LEFT SEMI JOIN orders o ON o.customer_id = c.id;",
			want: []string{"name=Alice", "name=Bob", "name=Charlie"},
		},
		// CROSS: 5 customers x 5 orders = 25 rows.
		{
			name: "cross",
			sql:  "SELECT c.name, o.order_id FROM customers c CROSS JOIN orders o;",
			want: nil, // checked by count below
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rows := queryJoin(t, eng, tc.sql)
			if tc.want == nil {
				if len(rows) != 25 {
					t.Fatalf("cross join: got %d rows, want 25", len(rows))
				}
				return
			}
			got := sortedKeys(rows, keysOf(tc.want)...)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d rows, want %d\ngot:  %v\nwant: %v", len(got), len(tc.want), got, tc.want)
			}
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("row %d mismatch:\ngot:  %s\nwant: %s", i, got[i], want[i])
				}
			}
		})
	}
}

// keysOf infers the column order from the first expected key ("col=val,...").
func keysOf(want []string) []string {
	if len(want) == 0 {
		return nil
	}
	pairs := strings.Split(want[0], ",")
	cols := make([]string, len(pairs))
	for i, p := range pairs {
		cols[i] = strings.SplitN(p, "=", 2)[0]
	}
	return cols
}

// TestSampleAllJoinsSQL executes samples/test_all_joins.sql end-to-end and
// asserts the COUNT for every join type matches the hand-computed value.
func TestSampleAllJoinsSQL(t *testing.T) {
	if err := os.Chdir(rootDir); err != nil {
		t.Fatalf("chdir to %s failed: %v", rootDir, err)
	}
	data, err := os.ReadFile("samples/test_all_joins.sql")
	if err != nil {
		t.Fatalf("read test_all_joins.sql failed: %v", err)
	}

	cat := catalog.NewCatalog()
	eng := engine.NewEngine(cat)
	p := parser.NewParser()
	stmts, err := p.Parse(string(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	wantCounts := map[string]string{
		"inner": "4", "left": "6", "left_outer": "6",
		"right": "5", "right_outer": "5",
		"full": "7", "full_outer": "7",
		"left_semi": "3", "cross": "25",
	}
	seen := map[string]bool{}
	for _, stmt := range stmts {
		if s, ok := stmt.(*parser.SelectStmt); ok {
			rows, err := eng.ExecuteSelect(s.Query)
			if err != nil {
				t.Fatalf("select failed: %v", err)
			}
			if len(rows) > 0 {
				if jt, ok := rows[0]["join_type"]; ok && jt != "" {
					seen[jt] = true
					if want, exists := wantCounts[jt]; !exists {
						t.Errorf("unexpected join_type %q", jt)
					} else if got := rows[0]["n"]; got != want {
						t.Errorf("join %s: COUNT = %s, want %s", jt, got, want)
					}
				}
			}
		} else if err := eng.Execute(stmt); err != nil {
			t.Fatalf("execute failed: %v", err)
		}
	}
	for jt := range wantCounts {
		if !seen[jt] {
			t.Errorf("join_type %q never produced by sample SQL", jt)
		}
	}
}
