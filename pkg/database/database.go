package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type Row = serde.Row

// Database defines the interface for SQL database operations.
type Database interface {
	Query(query string) ([]Row, error)
	Exec(query string, args ...any) error
	Close() error
}

type sqlDB struct {
	db     *sql.DB
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

// Open creates a Database connection from table options.
// If storage is not set, it is inferred from the url scheme.
func Open(tbl *catalog.Table) (Database, error) {
	storageType := strings.ToLower(tbl.Option("storage", ""))
	if storageType == "" {
		storageType = InferStorageFromURL(tbl.Option("url", ""))
	}
	driver := driverFor(storageType)
	if driver == "" {
		return nil, fmt.Errorf("unsupported database storage type: %q", storageType)
	}
	dsn := dataSourceName(tbl)
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", storageType, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping %s: %w", storageType, err)
	}
	return &sqlDB{db: db, dialect: storageType}, nil
}

// ReadTable reads rows from a database table, using optional custom query.
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

	// Filter to defined columns if table has them
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

// WriteTable writes rows to a database table, creating the table if needed.
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

	// Build columns from table definition or row keys
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
		switch storageType {
		case "sqlite", "sqlite3":
			d.Exec(fmt.Sprintf("DELETE FROM %s", tableName))
		default:
			d.Exec(fmt.Sprintf("TRUNCATE TABLE %s", tableName))
		}
	}

	colNames := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, c := range columns {
		colNames[i] = c.Name
		switch storageType {
		case "postgres", "postgresql":
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		default:
			placeholders[i] = "?"
		}
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

func driverFor(storageType string) string {
	switch storageType {
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	}
	return ""
}

func dataSourceName(tbl *catalog.Table) string {
	rawURL := tbl.Option("url", "")
	if rawURL != "" {
		return parseDSN(rawURL)
	}
	storageType := strings.ToLower(tbl.Option("storage", ""))
	switch storageType {
	case "mysql":
		h := tbl.Option("host", "127.0.0.1")
		p := tbl.Option("port", "3306")
		u := tbl.Option("username", "root")
		pw := tbl.Option("password", "")
		db := tbl.Option("database", "")
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true", u, pw, h, p, db)
	case "postgres", "postgresql":
		h := tbl.Option("host", "127.0.0.1")
		p := tbl.Option("port", "5432")
		u := tbl.Option("username", "postgres")
		pw := tbl.Option("password", "")
		db := tbl.Option("database", "")
		ssl := tbl.Option("sslmode", "disable")
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", h, p, u, pw, db, ssl)
	case "sqlite", "sqlite3":
		path := tbl.Option("path", tbl.Option("location", ""))
		if path == "" {
			path = tbl.Option("database", "gsql.db")
		}
		return path
	}
	return rawURL
}

func parseDSN(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	switch u.Scheme {
	case "mysql":
		user := u.User.Username()
		pass, _ := u.User.Password()
		host := u.Host
		db := strings.TrimPrefix(u.Path, "/")
		return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true", user, pass, host, db)
	case "postgres", "postgresql":
		user := u.User.Username()
		pass, _ := u.User.Password()
		host := u.Hostname()
		port := u.Port()
		db := strings.TrimPrefix(u.Path, "/")
		ssl := u.Query().Get("sslmode")
		if ssl == "" {
			ssl = "disable"
		}
		return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", host, port, user, pass, db, ssl)
	case "sqlite", "sqlite3":
		return strings.TrimPrefix(u.Path, "/")
	}
	return rawURL
}

// IsDatabase returns true if the storage type is a supported database.
func IsDatabase(storageType string) bool {
	if storageType == "" {
		return false
	}
	switch storageType {
	case "mysql", "postgres", "postgresql", "sqlite", "sqlite3":
		return true
	}
	return false
}

func InferStorageFromURL(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	switch u.Scheme {
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	}
	return ""
}
