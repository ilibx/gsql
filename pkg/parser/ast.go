package parser

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
	External    bool
	PartitionBy []string
}

func (c *CreateTableStmt) StatementName() string {
	if c.External {
		return "CREATE_EXTERNAL_TABLE"
	}
	return "CREATE_TABLE"
}

type InsertOverwriteStmt struct {
	TableName string
	Query     *SelectQuery
	Append    bool
}

func (i *InsertOverwriteStmt) StatementName() string {
	if i.Append {
		return "INSERT_INTO"
	}
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

type LogicalExpr struct {
	Left     Expression
	Operator string // "AND" or "OR"
	Right    Expression
}

func (e *LogicalExpr) expressionNode() {}

type AggregateExpr struct {
	FuncName string // COUNT, SUM, AVG, MIN, MAX
	Column   string // column name or "*"
}

type CTE struct {
	Name  string
	Query *SelectQuery
}

type JoinClause struct {
	RightTable  string
	RightAlias  string
	LeftColumn  string
	RightColumn string
}

type NullTestExpr struct {
	Column string
	IsNull bool // true for IS NULL, false for IS NOT NULL
}

func (e *NullTestExpr) expressionNode() {}

type InExpr struct {
	Column string
	Not    bool // true for NOT IN
	Values []string
}

func (e *InExpr) expressionNode() {}

type BinaryExpr struct {
	Left     Expression
	Operator string // +, -, *, /
	Right    Expression
}

func (e *BinaryExpr) expressionNode() {}

type ColumnRef struct {
	Name string
}

func (e *ColumnRef) expressionNode() {}

type CaseBranch struct {
	Condition Expression
	Result    Expression
}

type CaseExpr struct {
	Branches []CaseBranch
	Else     Expression
}

func (e *CaseExpr) expressionNode() {}

type SortOrder struct {
	Column string
	Desc   bool
}

type WindowExpr struct {
	FuncName    string     // ROW_NUMBER, RANK, DENSE_RANK, SUM, AVG, MIN, MAX, COUNT
	Args        []string   // function arguments (e.g., ["col"] for SUM(col))
	PartitionBy []string   // PARTITION BY columns
	OrderBy     []SortOrder // ORDER BY within window
}

func (e *WindowExpr) expressionNode() {}

type SelectQuery struct {
	CTEs          []CTE
	Columns       []string
	ColumnExprs   []Expression // parallel to Columns, nil means plain column ref
	Distinct      bool
	Table         string
	TableAlias    string
	FromSubquery  *SelectQuery
	FromAlias     string
	Joins         []JoinClause
	Where         Expression
	GroupBy       []string
	Having        Expression
	OrderBy       []SortOrder
	Limit         int
	Aggregates    []AggregateExpr
	WindowExprs   []WindowExpr  // parallel to Columns, nil means no window function
	ColumnAliases map[string]string
	UnionQuery    *SelectQuery
	UnionAll      bool
}
