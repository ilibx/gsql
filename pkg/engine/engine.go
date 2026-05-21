package engine

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/plan"
	"github.com/ilibx/gsql/pkg/sqlparse"
	"github.com/ilibx/gsql/pkg/storage"
)

type Engine struct {
	catalog *catalog.Catalog
}

type cteDef struct {
	Name  string
	Query string
}

type selectQuery struct {
	CTEs    []cteDef
	Columns []string
	Table   string
	Where   string
	OrderBy []string
	Limit   int
}

func NewEngine(catalog *catalog.Catalog) *Engine {
	return &Engine{catalog: catalog}
}

func (e *Engine) Execute(stmt sqlparse.Statement) error {
	switch node := stmt.(type) {
	case *sqlparse.CreateTableStmt:
		return e.executeCreateTable(node)
	case *sqlparse.InsertOverwriteStmt:
		return e.executeInsertOverwrite(node)
	case *sqlparse.SelectStmt:
		rows, err := e.executeSelect(node.Query)
		if err != nil {
			return err
		}
		printRows(rows)
		return nil
	default:
		return fmt.Errorf("unsupported statement type: %T", node)
	}
}

func (e *Engine) executeCreateTable(stmt *sqlparse.CreateTableStmt) error {
	columns := make([]catalog.ColumnDef, 0, len(stmt.Columns))
	for _, col := range stmt.Columns {
		columns = append(columns, catalog.ColumnDef{Name: col.Name, Type: col.Type})
	}
	table := &catalog.Table{
		Name:        stmt.Name,
		Columns:     columns,
		WithOptions: stmt.WithOptions,
	}
	if err := e.catalog.CreateTable(table); err != nil {
		return err
	}
	fmt.Printf("created table %s with %d columns\n", stmt.Name, len(stmt.Columns))
	return nil
}

func (e *Engine) executeInsertOverwrite(stmt *sqlparse.InsertOverwriteStmt) error {
	target, ok := e.catalog.GetTable(stmt.TableName)
	if !ok {
		return fmt.Errorf("target table %s does not exist", stmt.TableName)
	}

	rows, err := e.executeSelect(stmt.Query)
	if err != nil {
		return err
	}

	if err := storage.WriteRows(target, rows); err != nil {
		return fmt.Errorf("write target table %s failed: %w", stmt.TableName, err)
	}
	fmt.Printf("wrote %d rows to table %s\n", len(rows), stmt.TableName)
	return nil
}

func (e *Engine) executeSelect(query string) ([]storage.Row, error) {
	return e.executeSelectWithCTEs(query, make(map[string][]storage.Row))
}

func (e *Engine) executeSelectWithCTEs(query string, cteTables map[string][]storage.Row) ([]storage.Row, error) {
	sel, err := parseSelectQuery(query)
	if err != nil {
		return nil, err
	}
	for _, cte := range sel.CTEs {
		rows, err := e.executeSelectWithCTEs(cte.Query, cteTables)
		if err != nil {
			return nil, err
		}
		cteTables[strings.ToLower(cte.Name)] = cloneRows(rows)
	}

	root, err := e.buildPlan(sel, cteTables)
	if err != nil {
		return nil, err
	}
	return root.Execute()
}

func (e *Engine) buildPlan(sel *selectQuery, cteTables map[string][]storage.Row) (plan.PlanNode, error) {
	var root plan.PlanNode
	if rows, ok := cteTables[strings.ToLower(sel.Table)]; ok {
		root = plan.NewCTETableScanNode(sel.Table, rows)
	} else {
		table, ok := e.catalog.GetTable(sel.Table)
		if !ok {
			return nil, fmt.Errorf("table %s not found", sel.Table)
		}
		root = plan.NewTableScanNode(table)
	}

	if sel.Where != "" {
		root = plan.NewFilterNode(root, sel.Where)
	}
	if len(sel.OrderBy) > 0 {
		root = plan.NewSortNode(root, sel.OrderBy)
	}
	if sel.Limit > 0 {
		root = plan.NewLimitNode(root, sel.Limit)
	}
	if len(sel.Columns) > 0 {
		root = plan.NewProjectNode(root, sel.Columns)
	}
	return root, nil
}

func cloneRows(rows []storage.Row) []storage.Row {
	cloned := make([]storage.Row, 0, len(rows))
	for _, row := range rows {
		newRow := make(storage.Row)
		for k, v := range row {
			newRow[k] = v
		}
		cloned = append(cloned, newRow)
	}
	return cloned
}

func parseSelectQuery(sql string) (*selectQuery, error) {
	cleaned := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(sql), ";"))
	ctes, mainSQL, err := parseWithClause(cleaned)
	if err != nil {
		return nil, err
	}
	if mainSQL != "" {
		cleaned = mainSQL
	}
	lowerSQL := strings.ToLower(cleaned)
	if !strings.HasPrefix(lowerSQL, "select ") {
		return nil, fmt.Errorf("invalid select statement")
	}

	fromIndex := strings.Index(lowerSQL, " from ")
	if fromIndex == -1 {
		return nil, fmt.Errorf("missing FROM clause")
	}

	columnsPart := strings.TrimSpace(cleaned[6:fromIndex])
	rest := strings.TrimSpace(cleaned[fromIndex+6:])
	tableName := ""
	whereClause := ""
	orderBy := []string{}
	limit := 0

	tokens := strings.Fields(rest)
	if len(tokens) == 0 {
		return nil, fmt.Errorf("missing table name")
	}
	tableName = tokens[0]
	remainder := strings.TrimSpace(rest[len(tableName):])

	remainderLower := strings.ToLower(remainder)
	whereIndex := strings.Index(remainderLower, " where ")
	if whereIndex == -1 && strings.HasPrefix(remainderLower, "where ") {
		whereIndex = 0
	}
	orderIndex := strings.Index(remainderLower, " order by ")
	if orderIndex == -1 && strings.HasPrefix(remainderLower, "order by ") {
		orderIndex = 0
	}
	limitIndex := strings.Index(remainderLower, " limit ")
	if limitIndex == -1 && strings.HasPrefix(remainderLower, "limit ") {
		limitIndex = 0
	}

	if whereIndex >= 0 {
		if orderIndex >= 0 {
			whereClause = strings.TrimSpace(remainder[whereIndex+6 : orderIndex])
		} else if limitIndex >= 0 {
			whereClause = strings.TrimSpace(remainder[whereIndex+6 : limitIndex])
		} else {
			whereClause = strings.TrimSpace(remainder[whereIndex+6:])
		}
	}
	if orderIndex >= 0 {
		end := len(remainder)
		if limitIndex >= 0 {
			end = limitIndex
		}
		orderByClause := strings.TrimSpace(remainder[orderIndex+9 : end])
		orderBy = strings.Split(orderByClause, ",")
		for i := range orderBy {
			orderBy[i] = strings.TrimSpace(orderBy[i])
		}
	}
	if limitIndex >= 0 {
		limitClause := strings.TrimSpace(remainder[limitIndex+6:])
		if limitClause != "" {
			value, err := strconv.Atoi(limitClause)
			if err != nil {
				return nil, fmt.Errorf("invalid limit value: %w", err)
			}
			limit = value
		}
	}

	columns := []string{"*"}
	if columnsPart != "*" {
		cols := strings.Split(columnsPart, ",")
		columns = make([]string, 0, len(cols))
		for _, col := range cols {
			columns = append(columns, strings.TrimSpace(col))
		}
	}

	return &selectQuery{
		CTEs:    ctes,
		Columns: columns,
		Table:   tableName,
		Where:   whereClause,
		OrderBy: orderBy,
		Limit:   limit,
	}, nil
}

func parseWithClause(raw string) ([]cteDef, string, error) {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) < 5 || strings.ToLower(trimmed[:4]) != "with" {
		return nil, raw, nil
	}

	payload := strings.TrimSpace(trimmed[4:])
	var ctes []cteDef
	for {
		payload = strings.TrimSpace(payload)
		if payload == "" {
			return nil, "", fmt.Errorf("invalid WITH clause")
		}

		asIndex := strings.Index(strings.ToLower(payload), " as (")
		if asIndex < 0 {
			return nil, "", fmt.Errorf("invalid WITH clause, missing AS")
		}
		name := strings.TrimSpace(payload[:asIndex])
		payload = strings.TrimSpace(payload[asIndex+5:])

		depth := 1
		idx := 0
		for idx < len(payload) && depth > 0 {
			switch payload[idx] {
			case '(':
				depth++
			case ')':
				depth--
			}
			idx++
		}
		if depth != 0 {
			return nil, "", fmt.Errorf("unbalanced parentheses in WITH clause")
		}

		body := strings.TrimSpace(payload[:idx-1])
		ctes = append(ctes, cteDef{Name: name, Query: body})
		payload = strings.TrimSpace(payload[idx:])

		if strings.HasPrefix(strings.ToLower(payload), ",") {
			payload = strings.TrimSpace(payload[1:])
			continue
		}
		if strings.HasPrefix(strings.ToLower(payload), "select") {
			return ctes, payload, nil
		}
		if payload == "" {
			return nil, "", fmt.Errorf("missing final SELECT statement in WITH clause")
		}
		return nil, "", fmt.Errorf("invalid WITH clause syntax")
	}
}

func filterRows(rows []storage.Row, where string) []storage.Row {
	where = strings.TrimSpace(where)
	if where == "" {
		return rows
	}
	re := regexp.MustCompile(`(?i)^(\w+)\s*=\s*'([^']*)'$`)
	matches := re.FindStringSubmatch(where)
	if len(matches) == 3 {
		key := matches[1]
		value := matches[2]
		filtered := make([]storage.Row, 0, len(rows))
		for _, row := range rows {
			if row[key] == value {
				filtered = append(filtered, row)
			}
		}
		return filtered
	}
	fmt.Printf("DEBUG filterRows failed to parse where=%q\n", where)
	return rows
}

func sortRows(rows []storage.Row, orderBy []string) {
	if len(orderBy) == 0 {
		return
	}
	key := orderBy[0]
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i][key] < rows[j][key]
	})
}

func printRows(rows []storage.Row) {
	if len(rows) == 0 {
		fmt.Println("No rows returned")
		return
	}
	keys := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	fmt.Println(strings.Join(keys, ", "))
	for _, row := range rows {
		values := make([]string, len(keys))
		for i, key := range keys {
			values[i] = row[key]
		}
		fmt.Println(strings.Join(values, ", "))
	}
}
