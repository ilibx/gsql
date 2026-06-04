package database

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/serde"
	"github.com/ilibx/gsql/pkg/tunnel"
)

type Row = serde.Row

type Database interface {
	Query(query string) ([]Row, error)
	Exec(query string, args ...any) error
	Close() error
}

type sqlDB struct {
	db          *sql.DB
	dialect     string
	tunnelClose func()
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
	if d.tunnelClose != nil {
		d.tunnelClose()
	}
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

	// SSH tunnel for database connections behind a jump host
	var tunnelClose func()
	if sshCfg := tunnel.OptionsFromMap(tbl.WithOptions); sshCfg != nil {
		host, port, err := targetHostPort(tbl, storageType)
		if err != nil {
			return nil, fmt.Errorf("ssh tunnel: %w", err)
		}
		localAddr, closeFn, err := tunnel.Dial(sshCfg, host, port)
		if err != nil {
			return nil, fmt.Errorf("ssh tunnel: %w", err)
		}
		tunnelClose = closeFn
		dsn = rewriteDSN(dsn, storageType, localAddr)
	}

	db, err := sql.Open(info.driverName, dsn)
	if err != nil {
		if tunnelClose != nil {
			tunnelClose()
		}
		return nil, fmt.Errorf("failed to connect to %s: %w", storageType, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		if tunnelClose != nil {
			tunnelClose()
		}
		return nil, fmt.Errorf("failed to ping %s: %w", storageType, err)
	}
	return &sqlDB{db: db, dialect: storageType, tunnelClose: tunnelClose}, nil
}

// targetHostPort extracts the target host and port for SSH tunneling from
// table options or URL.
func targetHostPort(tbl *catalog.Table, storageType string) (string, int, error) {
	host := tbl.Option("host", "")
	portStr := tbl.Option("port", "")

	// Try parsed URL if no explicit host
	if host == "" {
		if rawURL := tbl.Option("url", ""); rawURL != "" {
			u, err := url.Parse(rawURL)
			if err == nil {
				host = u.Hostname()
				if p := u.Port(); p != "" {
					portStr = p
				}
			}
		}
	}

	if host == "" {
		return "", 0, fmt.Errorf("cannot determine target host; specify ssh_target_host or host/url")
	}

	port := defaultPort(storageType)
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// Allow override via ssh_target_host / ssh_target_port
	if h := tbl.Option("ssh_target_host", ""); h != "" {
		host = h
	}
	if p := tbl.Option("ssh_target_port", ""); p != "" {
		if pi, err := strconv.Atoi(p); err == nil {
			port = pi
		}
	}

	return host, port, nil
}

func defaultPort(storageType string) int {
	switch storageType {
	case "mysql":
		return 3306
	case "postgres", "postgresql":
		return 5432
	default:
		return 3306
	}
}

// rewriteDSN modifies the DSN to point to localAddr instead of the original host:port.
func rewriteDSN(dsn, storageType, localAddr string) string {
	switch storageType {
	case "mysql":
		// user:pass@tcp(host:port)/dbname?params -> user:pass@tcp(localAddr)/dbname?params
		if idx := strings.Index(dsn, "@tcp("); idx >= 0 {
			rest := dsn[idx+5:] // after "@tcp("
			if end := strings.Index(rest, ")"); end >= 0 {
				return dsn[:idx+5] + localAddr + rest[end:]
			}
		}
	case "postgres", "postgresql":
		// host=xxx port=yyy ... -> host=127.0.0.1 port=tunnelPort ...
		parts := strings.Fields(dsn)
		for i, p := range parts {
			if strings.HasPrefix(p, "host=") {
				parts[i] = "host=127.0.0.1"
			} else if strings.HasPrefix(p, "port=") {
				_, portStr, _ := strings.Cut(localAddr, ":")
				parts[i] = "port=" + portStr
			}
		}
		return strings.Join(parts, " ")
	}
	return dsn
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
