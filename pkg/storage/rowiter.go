package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/ilibx/gsql/pkg/catalog"
	"github.com/ilibx/gsql/pkg/database"
	"github.com/ilibx/gsql/pkg/serde"
)

// RowIter is a streaming row reader. Next returns io.EOF when the input is
// exhausted. It lets join/probe operate on batches without materializing
// entire tables.
type RowIter interface {
	Next() (Row, error)
	Close() error
}

// OpenTableIter opens a streaming iterator over a table's rows. Unlike
// ReadTableRows it never materializes the full table: files are opened and
// decoded one at a time on demand.
func OpenTableIter(tbl *catalog.Table, filters ...PartitionFilter) (RowIter, error) {
	if database.IsDatabase(storageType(tbl)) {
		rows, err := database.ReadTable(tbl)
		if err != nil {
			return nil, err
		}
		return &sliceRowIter{rows: rows}, nil
	}
	store, err := GetStorage(tbl)
	if err != nil {
		return nil, err
	}

	format := tbl.Option("format", "")
	if format == "" {
		closeStore(store)
		return nil, fmt.Errorf("missing format for table %s", tbl.Name)
	}
	pattern := tbl.Option("file_pattern", "")
	if pattern == "" {
		pattern = tbl.Option("file_name", "")
	}
	if pattern == "" {
		pattern = defaultPattern(format)
	}

	var files []partitionFile
	isPartitioned := len(tbl.PartitionBy) > 0
	if isPartitioned {
		bareFormat := tbl.Option("partition_format", "") == "value"
		files, err = resolvePartitionPaths(store, ".", pattern, tbl.PartitionBy, filters, bareFormat)
	} else {
		var paths []string
		paths, err = resolveLocalPaths(store, pattern)
		if err == nil {
			files = make([]partitionFile, len(paths))
			for i, p := range paths {
				files[i] = partitionFile{Path: p}
			}
		}
	}
	if err != nil {
		closeStore(store)
		return nil, err
	}
	if len(files) == 0 {
		closeStore(store)
		return nil, fmt.Errorf("no files found for table %s matching pattern %q", tbl.Name, pattern)
	}

	return &fileRowIter{
		store:       store,
		format:      format,
		columns:     tbl.Columns,
		opts:        serde.NewSerdeOptions(tbl),
		files:       files,
		partitioned: isPartitioned,
	}, nil
}

// closeStore closes a Storage that implements Close, mirroring ReadTableRows.
func closeStore(store Storage) {
	if c, ok := store.(interface{ Close() error }); ok {
		c.Close()
	}
}

func defaultPattern(format string) string {
	switch format {
	case "csv":
		return "*.csv"
	case "json":
		return "*.json"
	case "excel", "xlsx":
		return "*.xlsx"
	default:
		return "*"
	}
}

// fileRowIter walks a table's files one at a time, decoding rows on demand.
type fileRowIter struct {
	store       Storage
	format      string
	columns     []catalog.ColumnDef
	opts        serde.SerdeOptions
	files       []partitionFile
	partitioned bool

	idx     int
	decoder serde.Decoder
	file    io.Closer
	closed  bool
}

func (it *fileRowIter) Next() (Row, error) {
	if it.closed {
		return nil, io.EOF
	}
	for {
		if it.decoder == nil {
			if it.idx >= len(it.files) {
				return nil, io.EOF
			}
			pf := it.files[it.idx]
			file, err := it.store.Open(context.Background(), pf.Path)
			if err != nil {
				return nil, err
			}
			dec, err := serde.NewDecoder(it.format, file, it.columns, it.opts)
			if err != nil {
				file.Close()
				return nil, err
			}
			it.decoder = dec
			it.file = file // keep open until this file is fully decoded
		}
		row, err := it.decoder.Next()
		if err == io.EOF {
			it.decoder.Close()
			it.decoder = nil
			if it.file != nil {
				it.file.Close()
				it.file = nil
			}
			it.idx++
			continue
		}
		if err != nil {
			return nil, err
		}
		if it.partitioned {
			for k, v := range it.files[it.idx].PartitionValues {
				row[k] = v
			}
		}
		return row, nil
	}
}

func (it *fileRowIter) Close() error {
	if it.closed {
		return nil
	}
	it.closed = true
	if it.decoder != nil {
		it.decoder.Close()
		it.decoder = nil
	}
	if it.file != nil {
		it.file.Close()
		it.file = nil
	}
	closeStore(it.store)
	return nil
}

// sliceRowIter replays an already-materialized row set (database tables).
type sliceRowIter struct {
	rows []Row
	pos  int
}

func (it *sliceRowIter) Next() (Row, error) {
	if it.pos >= len(it.rows) {
		return nil, io.EOF
	}
	row := it.rows[it.pos]
	it.pos++
	return row, nil
}

func (it *sliceRowIter) Close() error { return nil }
