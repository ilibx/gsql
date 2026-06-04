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

	// URL scheme 优先（确定性的，不受 map 遍历顺序影响）
	switch u.Scheme {
	case "mysql":
		return "mysql"
	case "postgres", "postgresql":
		return "postgres"
	case "sqlite", "sqlite3":
		return "sqlite"
	}

	// 非标准 scheme 回退到 parseURL 探测
	for name, info := range registry {
		if info.parseURL != nil {
			dsn := info.parseURL(u)
			if dsn != rawURL {
				return name
			}
		}
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
