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
	Values    [][]string // Values for INSERT INTO ... VALUES
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

type ExplainStmt struct {
	Query *SelectQuery
}

func (e *ExplainStmt) StatementName() string {
	return "EXPLAIN"
}

type Expression interface {
	expressionNode()
}

type ComparisonExpr struct {
	Column      string
	Operator    string
	Value       string
	RightColumn string // set for column-to-column comparisons (WHERE a = b)
	Expr        Expression // set for computed expressions (WHERE age + 5 > 25)
}

func (e *ComparisonExpr) expressionNode() {}

type LogicalExpr struct {
	Left     Expression
	Operator string // "AND" or "OR"
	Right    Expression
}

func (e *LogicalExpr) expressionNode() {}

type AggregateExpr struct {
	FuncName string   // COUNT, SUM, AVG, MIN, MAX
	Column   string   // column name or "*" (first arg, for backward compat)
	Args     []string // all arguments (for multi-arg agg like CORR(x,y))
	Distinct bool     // true for COUNT(DISTINCT col)
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
	Column   string
	Not      bool // true for NOT IN
	Values   []string
	Subquery *SelectQuery // non-nil for IN (SELECT ...)
}

func (e *InExpr) expressionNode() {}

type ExistsExpr struct {
	Not      bool // true for NOT EXISTS
	Subquery *SelectQuery
}

func (e *ExistsExpr) expressionNode() {}

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

// LiteralExpr represents a literal value in a SELECT expression (e.g., '2026' AS year)
type LiteralExpr struct {
	Value string
}

func (e *LiteralExpr) expressionNode() {}

// FuncCallExpr represents a scalar function call (e.g., UPPER(name), LENGTH(col))
type FuncCallExpr struct {
	FuncName string
	Args     []string
}

func (e *FuncCallExpr) expressionNode() {}

type SortOrder struct {
	Column string
	Desc   bool
}

type WindowFrameBound string

const (
	FrameUnboundedPreceding WindowFrameBound = "UNBOUNDED PRECEDING"
	FrameNPreceding         WindowFrameBound = "PRECEDING"
	FrameCurrentRow         WindowFrameBound = "CURRENT ROW"
	FrameNFollowing         WindowFrameBound = "FOLLOWING"
	FrameUnboundedFollowing WindowFrameBound = "UNBOUNDED FOLLOWING"
)

type WindowFrameType string

const (
	FrameRows  WindowFrameType = "ROWS"
	FrameRange WindowFrameType = "RANGE"
)

type WindowFrameBoundary struct {
	BoundType WindowFrameBound
	N         int    // for n PRECEDING / n FOLLOWING; 0 for UNBOUNDED / CURRENT ROW
}

type WindowFrame struct {
	FrameType WindowFrameType
	Start     WindowFrameBoundary
	End       WindowFrameBoundary
	OrderBy   string // ORDER BY column used for RANGE frame (set at execution time)
}

type WindowExpr struct {
	FuncName    string        // ROW_NUMBER, RANK, DENSE_RANK, SUM, AVG, MIN, MAX, COUNT
	Args        []string      // function arguments (e.g., ["col"] for SUM(col))
	PartitionBy []string      // PARTITION BY columns
	OrderBy     []SortOrder   // ORDER BY within window
	Frame       *WindowFrame  // nil means default frame (RANGE BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW with ORDER BY)
}

func (e *WindowExpr) expressionNode() {}

type LateralView struct {
	IsOuter   bool
	FuncName  string   // EXPLODE, POSEXPLODE
	Args      []string
	TableName string   // alias for the generated table
	ColNames  []string // column aliases (e.g., ["val"] for EXPLODE(arr), ["pos","val"] for POSEXPLODE, ["key","val"] for EXPLODE(map))
}

type SelectQuery struct {
	CTEs          []CTE
	Columns       []string
	ColumnExprs   []Expression // parallel to Columns, nil means plain column ref
	Distinct      bool
	Table         string
	TableAlias    string
	FromSubquery  *SelectQuery
	FromAlias     string
	FromValues    [][]string // VALUES rows in FROM clause
	FromValuesCols []string  // column aliases from VALUES AS t(col1, col2, ...)
	Joins         []JoinClause
	Where         Expression
	GroupBy       []string
	Having        Expression
	OrderBy       []SortOrder
	Limit         int
	HasLimit      bool
	Aggregates    []AggregateExpr
	WindowExprs   []WindowExpr  // parallel to Columns, nil means no window function
	ColumnAliases map[string]string
	UnionQuery    *SelectQuery
	UnionAll      bool
	LateralViews  []LateralView
}
