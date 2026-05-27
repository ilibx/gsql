package database

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
	_ "github.com/go-sql-driver/mysql"
)

func init() {
	RegisterDriver("mysql", driverInfo{
		driverName: "mysql",
		buildDSN:   buildMySQLDSN,
		parseURL:   parseMySQLURL,
	})
}

func buildMySQLDSN(tbl *catalog.Table) string {
	h := tbl.Option("host", "127.0.0.1")
	p := tbl.Option("port", "3306")
	u := tbl.Option("username", "root")
	pw := tbl.Option("password", "")
	db := tbl.Option("database", "")
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=true", u, pw, h, p, db)
}

func parseMySQLURL(u *url.URL) string {
	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Host
	db := strings.TrimPrefix(u.Path, "/")
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true", user, pass, host, db)
}
