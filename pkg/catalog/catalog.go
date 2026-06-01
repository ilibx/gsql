package catalog

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

// DebugLevel controls the verbosity of debug output.
// 0 = silent, 1 = basic info, 2+ = more detail.
var DebugLevel int

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
	
	if DebugLevel > 0 {
		fmt.Printf("DEBUG: Creating table %s with options: %v\n", tbl.Name, tbl.WithOptions)
	}
	// Parse URL and automatically fill in storage-related parameters
	parseStorageURL(tbl)
	if DebugLevel > 0 {
		fmt.Printf("DEBUG: After URL parsing, options: %v\n", tbl.WithOptions)
	}
	
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

// parseStorageURL parses the 'url' option and automatically fills in storage-related parameters.
func parseStorageURL(tbl *Table) {
    urlStr, hasURL := tbl.WithOptions["url"]
    if !hasURL {
        return // No URL parameter, keep original logic
    }
    if DebugLevel > 0 {
        fmt.Printf("DEBUG: Parsing URL: %s\n", urlStr)
    }
    
    parsed, err := url.Parse(urlStr)
    if err != nil {
        return // Parse failed, keep original logic
    }
    
    scheme := strings.ToLower(parsed.Scheme)
    switch scheme {
    case "ftp":
        tbl.WithOptions["storage"] = "ftp"
        tbl.WithOptions["host"] = parsed.Hostname()
        if parsed.Port() != "" {
            tbl.WithOptions["port"] = parsed.Port()
        }
        if parsed.User != nil {
            tbl.WithOptions["username"] = parsed.User.Username()
            if pass, ok := parsed.User.Password(); ok {
                tbl.WithOptions["password"] = pass
            }
        }
        tbl.WithOptions["path"] = parsed.Path
    case "sftp":
        tbl.WithOptions["storage"] = "sftp"
        tbl.WithOptions["host"] = parsed.Hostname()
        if parsed.Port() != "" {
            tbl.WithOptions["port"] = parsed.Port()
        }
        if parsed.User != nil {
            tbl.WithOptions["username"] = parsed.User.Username()
            if pass, ok := parsed.User.Password(); ok {
                tbl.WithOptions["password"] = pass
            }
        }
        tbl.WithOptions["path"] = parsed.Path
    case "s3":
        tbl.WithOptions["storage"] = "s3"
        // Format: s3://endpoint/bucket/path or s3://bucket/path (legacy)
        // If the host looks like an endpoint (has dots), use it as endpoint;
        // otherwise treat it as a bucket name (legacy format).
        pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
        if strings.Contains(parsed.Host, ".") || strings.Contains(parsed.Host, ":") {
            // Host is an endpoint: s3://s3.example.com/bucket/path
            tbl.WithOptions["endpoint"] = fmt.Sprintf("https://%s", parsed.Host)
            if len(pathParts) > 0 {
                tbl.WithOptions["bucket"] = pathParts[0]
            }
			if len(pathParts) > 1 {
				tbl.WithOptions["path"] = strings.Join(pathParts[1:], "/")
            }
        } else {
            // Host is a bucket name (legacy): s3://bucket/path
            tbl.WithOptions["bucket"] = parsed.Host
			if len(pathParts) > 0 && pathParts[0] != "" {
				tbl.WithOptions["path"] = strings.Join(pathParts, "/")
            }
        }
        // Parse query parameters
        for k, v := range parsed.Query() {
            if len(v) > 0 {
                key := strings.ToLower(k)
                if key == "access_key" || key == "accesskey" {
                    tbl.WithOptions["access_key"] = v[0]
                } else if key == "access_secret" || key == "accesssecret" || key == "secret_key" || key == "secretkey" {
                    tbl.WithOptions["access_secret"] = v[0]
                } else {
                    tbl.WithOptions["s3_"+key] = v[0]
                }
            }
        }
    case "hdfs":
        tbl.WithOptions["storage"] = "hdfs"
        tbl.WithOptions["path"] = parsed.Path
    case "webdav":
        tbl.WithOptions["storage"] = "webdav"
        tbl.WithOptions["url"] = fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, parsed.Path)
        if parsed.User != nil {
            tbl.WithOptions["username"] = parsed.User.Username()
            if pass, ok := parsed.User.Password(); ok {
                tbl.WithOptions["password"] = pass
            }
        }
    case "gitlfs", "git-lfs", "git":
        tbl.WithOptions["storage"] = "gitlfs"
        if strings.HasPrefix(parsed.Path, "/") {
            tbl.WithOptions["path"] = parsed.Path
        } else {
            tbl.WithOptions["repo"] = parsed.Path
        }
    case "lark":
        tbl.WithOptions["storage"] = "lark"
        tbl.WithOptions["root_token"] = parsed.Host
        if parsed.User != nil {
            tbl.WithOptions["app_id"] = parsed.User.Username()
            if pass, ok := parsed.User.Password(); ok {
                tbl.WithOptions["app_secret"] = pass
            }
        }
        for k, v := range parsed.Query() {
            if len(v) > 0 {
                key := strings.ToLower(k)
                tbl.WithOptions[key] = v[0]
            }
        }
    case "local":
        tbl.WithOptions["storage"] = "local"
		tbl.WithOptions["path"] = parsed.Path
    }
}
