package database

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	_ "github.com/lib/pq"
)

func init() {
	for _, name := range []string{"postgres", "postgresql"} {
		RegisterDriver(name, driverInfo{
			driverName: "postgres",
			buildDSN:   buildPostgresDSN,
			parseURL:   parsePostgresURL,
		})
	}
}

func buildPostgresDSN(tbl *catalog.Table) string {
	h := tbl.Option("host", "127.0.0.1")
	p := tbl.Option("port", "5432")
	u := tbl.Option("username", "postgres")
	pw := tbl.Option("password", "")
	db := tbl.Option("database", "")
	ssl := tbl.Option("sslmode", "disable")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", h, p, u, pw, db, ssl)
}

func parsePostgresURL(u *url.URL) string {
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
}
