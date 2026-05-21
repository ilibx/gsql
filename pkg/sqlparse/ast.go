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
    Query     string
}

func (i *InsertOverwriteStmt) StatementName() string {
    return "INSERT_OVERWRITE"
}

type SelectStmt struct {
    Query string
}

func (s *SelectStmt) StatementName() string {
    return "SELECT"
}
