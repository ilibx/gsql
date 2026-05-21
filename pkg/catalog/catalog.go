package catalog

import (
    "fmt"
    "strings"
)

// Table defines a logical table and its data source options.
type Table struct {
    Name        string
    Columns     []ColumnDef
    WithOptions map[string]string
}

type ColumnDef struct {
    Name string
    Type string
}

type Catalog struct {
    tables map[string]*Table
}

func NewCatalog() *Catalog {
    return &Catalog{tables: make(map[string]*Table)}
}

func (c *Catalog) CreateTable(tbl *Table) error {
    key := normalizeName(tbl.Name)
    if _, exists := c.tables[key]; exists {
        return fmt.Errorf("table %s already exists", tbl.Name)
    }
    c.tables[key] = tbl
    return nil
}

func (c *Catalog) GetTable(name string) (*Table, bool) {
    tbl, ok := c.tables[normalizeName(name)]
    return tbl, ok
}

func (t *Table) Option(key string, defaultValue string) string {
    if t.WithOptions == nil {
        return defaultValue
    }
    if v, ok := t.WithOptions[key]; ok {
        return v
    }
    return defaultValue
}

func normalizeName(name string) string {
    return strings.ToLower(name)
}
