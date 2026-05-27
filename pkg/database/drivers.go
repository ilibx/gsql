package database

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ilibx/gsql/pkg/catalog"
)

func dataSourceName(tbl *catalog.Table, info driverInfo) string {
	rawURL := tbl.Option("url", "")
	if rawURL != "" {
		return parseDSN(rawURL, info)
	}
	return info.buildDSN(tbl)
}

func parseDSN(rawURL string, info driverInfo) string {
	if !strings.Contains(rawURL, "://") {
		return rawURL
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if info.parseURL != nil {
		return info.parseURL(u)
	}
	return rawURL
}

func IsDatabase(storageType string) bool {
	if storageType == "" {
		return false
	}
	_, ok := registry[strings.ToLower(storageType)]
	return ok
}

func InferStorageFromURL(rawURL string) string {
	if !strings.Contains(rawURL, "://") {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	for name, info := range registry {
		if info.parseURL != nil {
			dsn := info.parseURL(u)
			if dsn != rawURL {
				return name
			}
		}
	}
	switch u.Scheme {
	case "mysql", "postgres", "postgresql", "sqlite", "sqlite3":
		return u.Scheme
	}
	return ""
}

func placeholder(storageType string, idx int) string {
	if storageType == "postgres" || storageType == "postgresql" {
		return fmt.Sprintf("$%d", idx+1)
	}
	return "?"
}

func clearTableSQL(storageType, tableName string) string {
	if storageType == "sqlite" || storageType == "sqlite3" {
		return fmt.Sprintf("DELETE FROM %s", tableName)
	}
	return fmt.Sprintf("TRUNCATE TABLE %s", tableName)
}
