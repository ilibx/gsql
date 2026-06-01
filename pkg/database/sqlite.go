package database

import (
	"net/url"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	_ "modernc.org/sqlite"
)

func init() {
	for _, name := range []string{"sqlite", "sqlite3"} {
		RegisterDriver(name, driverInfo{
			driverName: "sqlite",
			buildDSN:   buildSQLiteDSN,
			parseURL:   parseSQLiteURL,
		})
	}
}

func buildSQLiteDSN(tbl *catalog.Table) string {
	path := tbl.Option("path", "")
	if path == "" {
		path = tbl.Option("database", "gsql.db")
	}
	return path
}

func parseSQLiteURL(u *url.URL) string {
	return strings.TrimPrefix(u.Path, "/")
}
