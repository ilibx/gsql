package sqlparse

import (
    "fmt"
    "regexp"
    "strings"
)

type Parser struct{}

func NewParser() *Parser {
    return &Parser{}
}

func (p *Parser) Parse(sql string) ([]Statement, error) {
    cleaned := removeComments(sql)
    rawStatements := splitStatements(cleaned)
    var statements []Statement
    for _, raw := range rawStatements {
        raw = strings.TrimSpace(raw)
        if raw == "" {
            continue
        }
        lower := strings.ToLower(raw)
        switch {
        case strings.HasPrefix(lower, "create table"):
            stmt, err := parseCreateTable(raw)
            if err != nil {
                return nil, err
            }
            statements = append(statements, stmt)
        case strings.HasPrefix(lower, "insert overwrite table"):
            stmt, err := parseInsertOverwrite(raw)
            if err != nil {
                return nil, err
            }
            statements = append(statements, stmt)
        case strings.HasPrefix(lower, "select"):
            statements = append(statements, &SelectStmt{Query: raw})
        default:
            return nil, fmt.Errorf("unsupported statement: %s", raw)
        }
    }
    return statements, nil
}

func removeComments(sql string) string {
    lines := strings.Split(sql, "\n")
    var cleaned []string
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if strings.HasPrefix(line, "--") {
            continue
        }
        cleaned = append(cleaned, line)
    }
    return strings.Join(cleaned, "\n")
}

func splitStatements(sql string) []string {
    parts := strings.Split(sql, ";")
    var statements []string
    for _, part := range parts {
        statements = append(statements, part)
    }
    return statements
}

func parseCreateTable(raw string) (*CreateTableStmt, error) {
    normalized := strings.TrimSpace(raw)
    re := regexp.MustCompile(`(?is)^create\s+table\s+(\S+)\s*\((.*)\)\s*with\s*\((.*)\)$`)
    matches := re.FindStringSubmatch(normalized)
    if len(matches) != 4 {
        return nil, fmt.Errorf("invalid CREATE TABLE statement")
    }

    tableName := strings.TrimSpace(matches[1])
    columnsPart := strings.TrimSpace(matches[2])
    optionsPart := strings.TrimSpace(matches[3])

    columns, err := parseColumns(columnsPart)
    if err != nil {
        return nil, err
    }
    options, err := parseWithOptions(optionsPart)
    if err != nil {
        return nil, err
    }

    return &CreateTableStmt{
        Name:        tableName,
        Columns:     columns,
        WithOptions: options,
    }, nil
}

func parseColumns(raw string) ([]ColumnDef, error) {
    lines := splitColumns(raw)
    var columns []ColumnDef
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" {
            continue
        }
        parts := strings.Fields(line)
        if len(parts) < 2 {
            return nil, fmt.Errorf("invalid column definition: %s", line)
        }
        columns = append(columns, ColumnDef{Name: parts[0], Type: parts[1]})
    }
    return columns, nil
}

func splitColumns(raw string) []string {
    raw = strings.ReplaceAll(raw, "\r\n", "\n")
    return strings.Split(raw, ",")
}

func parseWithOptions(raw string) (map[string]string, error) {
    result := make(map[string]string)
    entries := splitOptions(raw)
    for _, entry := range entries {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        parts := strings.SplitN(entry, "=", 2)
        if len(parts) != 2 {
            return nil, fmt.Errorf("invalid WITH option: %s", entry)
        }
        key := strings.TrimSpace(parts[0])
        value := strings.TrimSpace(parts[1])
        value = strings.Trim(value, `"'`)
        result[key] = value
    }
    return result, nil
}

func splitOptions(raw string) []string {
    raw = strings.ReplaceAll(raw, "\r\n", "\n")
    parts := strings.Split(raw, ",")
    return parts
}

func parseInsertOverwrite(raw string) (*InsertOverwriteStmt, error) {
    lines := strings.Split(raw, "\n")
    if len(lines) == 0 {
        return nil, fmt.Errorf("invalid INSERT OVERWRITE statement")
    }
    firstLine := strings.TrimSpace(lines[0])
    prefix := "INSERT OVERWRITE TABLE"
    if !strings.HasPrefix(strings.ToUpper(firstLine), prefix) {
        return nil, fmt.Errorf("invalid INSERT OVERWRITE statement")
    }
    remainder := strings.TrimSpace(firstLine[len(prefix):])
    parts := strings.Fields(remainder)
    if len(parts) == 0 {
        return nil, fmt.Errorf("missing target table name")
    }
    tableName := parts[0]
    query := strings.TrimSpace(strings.Join(lines[1:], "\n"))
    if query == "" {
        return nil, fmt.Errorf("missing select query in INSERT OVERWRITE")
    }
    return &InsertOverwriteStmt{TableName: tableName, Query: query}, nil
}
