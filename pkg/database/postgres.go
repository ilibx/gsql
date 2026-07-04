package database

import (
	"fmt"
	"net/url"
	"strings"
	"time"

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

// localTimezone returns the machine's local timezone as a PostgreSQL-compatible
// offset string (e.g. "+08:00", "-05:00", "+05:30").
func localTimezone() string {
	_, offset := time.Now().Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	return fmt.Sprintf("%s%02d:%02d", sign, offset/3600, (offset%3600)/60)
}

func buildPostgresDSN(tbl *catalog.Table) string {
	h := tbl.Option("host", "127.0.0.1")
	p := tbl.Option("port", "5432")
	u := tbl.Option("username", "postgres")
	pw := tbl.Option("password", "")
	db := tbl.Option("database", "")
	ssl := tbl.Option("sslmode", "disable")
	tz := tbl.Option("timezone", "")
	if tz == "" {
		tz = localTimezone()
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s timezone=%s", h, p, u, pw, db, ssl, tz)
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
	tz := u.Query().Get("timezone")
	if tz == "" {
		tz = localTimezone()
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s timezone=%s", host, port, user, pass, db, ssl, tz)
}
