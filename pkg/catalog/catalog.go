package catalog

import (
	"fmt"
	"strings"
	"sync"
)

// Table defines a logical table and its data source options.
type Table struct {
	Name          string
	Columns       []ColumnDef
	WithOptions   map[string]string
	External      bool
	PartitionBy   []string
	EstimatedRows int64 // 0 means unknown
}

type ColumnDef struct {
    Name string
    Type string
}

type Catalog struct {
	mu     sync.RWMutex
	tables map[string]*Table
}

func NewCatalog() *Catalog {
    return &Catalog{tables: make(map[string]*Table)}
}

func (c *Catalog) CreateTable(tbl *Table) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	key := normalizeName(tbl.Name)
	if _, exists := c.tables[key]; exists {
		return fmt.Errorf("table %s already exists", tbl.Name)
	}
	c.tables[key] = tbl
	return nil
}

func (c *Catalog) GetTable(name string) (*Table, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	tbl, ok := c.tables[normalizeName(name)]
	return tbl, ok
}

func (c *Catalog) TableNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	names := make([]string, 0, len(c.tables))
	for n := range c.tables {
		names = append(names, n)
	}
	return names
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
