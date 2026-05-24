package catalog

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
)

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
	
	fmt.Printf("DEBUG: Creating table %s with options: %v\n", tbl.Name, tbl.WithOptions)
	// Parse URL and automatically fill in storage-related parameters
	parseStorageURL(tbl)
	fmt.Printf("DEBUG: After URL parsing, options: %v\n", tbl.WithOptions)
	
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
    fmt.Printf("DEBUG: Parsing URL: %s\n", urlStr)
    
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
        // Extract bucket name from path (first component)
        pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
        if len(pathParts) > 0 {
            tbl.WithOptions["bucket"] = pathParts[0]
        }
        if len(pathParts) > 1 {
            tbl.WithOptions["prefix"] = strings.Join(pathParts[1:], "/")
        }
        // Parse query parameters
        for k, v := range parsed.Query() {
            if len(v) > 0 {
                tbl.WithOptions["s3_" + strings.ToLower(k)] = v[0]
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
    case "local":
        tbl.WithOptions["storage"] = "local"
        tbl.WithOptions["location"] = parsed.Path
    }
}
