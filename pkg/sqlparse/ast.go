package sqlparse

type Statement interface {
	StatementName() string
}

type ColumnDef struct {
	Name string
	Type string
}

type CreateTableStmt struct {
	Name        string
	Columns     []ColumnDef
	WithOptions map[string]string
}

func (c *CreateTableStmt) StatementName() string {
	return "CREATE_TABLE"
}

type InsertOverwriteStmt struct {
	TableName string
	Query     *SelectQuery
}

func (i *InsertOverwriteStmt) StatementName() string {
	return "INSERT_OVERWRITE"
}

type SelectStmt struct {
	Query *SelectQuery
}

func (s *SelectStmt) StatementName() string {
	return "SELECT"
}

type Expression interface {
	expressionNode()
}

type ComparisonExpr struct {
	Column   string
	Operator string
	Value    string
}

func (e *ComparisonExpr) expressionNode() {}

type CTE struct {
	Name  string
	Query *SelectQuery
}

type SelectQuery struct {
	CTEs    []CTE
	Columns []string
	Table   string
	Where   Expression
	OrderBy []string
	Limit   int
}
