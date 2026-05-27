package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"
)

type Row = serde.Row

type Database interface {
	Query(query string) ([]Row, error)
	Exec(query string, args ...any) error
	Close() error
}

type sqlDB struct {
	db      *sql.DB
	dialect string
}

func (d *sqlDB) Query(query string) ([]Row, error) {
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns failed: %w", err)
	}

	var result []Row
	for rows.Next() {
		vals := make([]any, len(cols))
		valPtrs := make([]any, len(cols))
		for i := range cols {
			valPtrs[i] = &vals[i]
		}
		if err := rows.Scan(valPtrs...); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}
		row := make(Row)
		for i, col := range cols {
			if vals[i] == nil {
				row[col] = ""
			} else {
				switch v := vals[i].(type) {
				case []byte:
					row[col] = string(v)
				default:
					row[col] = fmt.Sprint(v)
				}
			}
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (d *sqlDB) Exec(query string, args ...any) error {
	_, err := d.db.Exec(query, args...)
	return err
}

func (d *sqlDB) Close() error {
	return d.db.Close()
}

type driverInfo struct {
	driverName string
	buildDSN   func(tbl *catalog.Table) string
	parseURL   func(u *url.URL) string
}

var registry = map[string]driverInfo{}

func RegisterDriver(name string, info driverInfo) {
	registry[strings.ToLower(name)] = info
}

func Open(tbl *catalog.Table) (Database, error) {
	storageType := strings.ToLower(tbl.Option("storage", ""))
	if storageType == "" {
		storageType = InferStorageFromURL(tbl.Option("url", ""))
	}
	info, ok := registry[storageType]
	if !ok {
		return nil, fmt.Errorf("unsupported database storage type: %q", storageType)
	}
	dsn := dataSourceName(tbl, info)
	db, err := sql.Open(info.driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", storageType, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping %s: %w", storageType, err)
	}
	return &sqlDB{db: db, dialect: storageType}, nil
}

func ReadTable(tbl *catalog.Table) ([]Row, error) {
	d, err := Open(tbl)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	query := tbl.Option("query", "")
	if query == "" {
		dbTable := tbl.Option("table_name", tbl.Name)
		query = fmt.Sprintf("SELECT * FROM %s", dbTable)
	}

	rows, err := d.Query(query)
	if err != nil {
		return nil, err
	}

	if len(tbl.Columns) > 0 {
		colSet := make(map[string]bool)
		for _, c := range tbl.Columns {
			colSet[c.Name] = true
		}
		pruned := make([]Row, len(rows))
		for i, r := range rows {
			nr := make(Row)
			for k, v := range r {
				if colSet[k] {
					nr[k] = v
				}
			}
			pruned[i] = nr
		}
		rows = pruned
	}
	return rows, nil
}

func WriteTable(tbl *catalog.Table, rows []Row, appendMode bool) error {
	if len(rows) == 0 {
		return nil
	}

	d, err := Open(tbl)
	if err != nil {
		return err
	}
	defer d.Close()

	storageType := strings.ToLower(tbl.Option("storage", ""))
	tableName := tbl.Option("table_name", tbl.Name)

	var columns []catalog.ColumnDef
	if len(tbl.Columns) > 0 {
		columns = tbl.Columns
	} else {
		for k := range rows[0] {
			columns = append(columns, catalog.ColumnDef{Name: k, Type: "STRING"})
		}
	}

	if err := ensureTable(d, tableName, columns); err != nil {
		return fmt.Errorf("create table failed: %w", err)
	}

	if !appendMode {
		d.Exec(clearTableSQL(storageType, tableName))
	}

	colNames := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, c := range columns {
		colNames[i] = c.Name
		placeholders[i] = placeholder(storageType, i)
	}

	colList := strings.Join(colNames, ", ")
	phList := strings.Join(placeholders, ", ")
	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tableName, colList, phList)

	for _, row := range rows {
		vals := make([]any, len(columns))
		for i, c := range columns {
			vals[i] = row[c.Name]
		}
		if err := d.Exec(insertSQL, vals...); err != nil {
			return fmt.Errorf("insert row failed: %w", err)
		}
	}
	return nil
}

func ensureTable(d Database, tableName string, columns []catalog.ColumnDef) error {
	colDefs := make([]string, len(columns))
	typeMap := map[string]string{
		"INT": "INTEGER", "BIGINT": "BIGINT", "FLOAT": "FLOAT",
		"DOUBLE": "FLOAT", "STRING": "TEXT", "VARCHAR": "TEXT",
		"CHAR": "TEXT", "BOOLEAN": "INTEGER",
	}
	for i, c := range columns {
		t := typeMap[c.Type]
		if t == "" {
			t = "TEXT"
		}
		colDefs[i] = fmt.Sprintf("%s %s", c.Name, t)
	}
	query := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", tableName, strings.Join(colDefs, ", "))
	return d.Exec(query)
}
