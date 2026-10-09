package plan

import (
	"io"
	"runtime"
	"sync"

	"github.com/ilibx/gsql/pkg/storage"
)

const (
	// joinProbeBatchSize is how many probe-side (left) rows are pulled from
	// the child iterator at a time. Probe memory is bounded to roughly
	// workers * joinProbeBatchSize rows instead of the whole left table.
	joinProbeBatchSize = 2048
	// joinQueueDepth bounds in-flight batches between the reader and workers.
	joinQueueDepth = 4
)

// joinIter executes a hash join in two stages:
//
//  1. build: stream the right (build) side once into a shared hash table
//     (read-only afterwards).
//  2. probe: stream the left side in batches; batches are dispatched to a
//     worker pool that probes the shared hash in parallel. Results carry the
//     batch sequence number and are re-ordered by Next, so output order is
//     identical to serial execution.
//
// Peak memory = build side + (workers + queue depth) * joinProbeBatchSize
// probe rows. The probe side is never fully materialized.
type joinIter struct {
	node      *JoinNode
	normalize func(string) string
	leftCol   string
	rightCol  string

	// build side, read-only after init
	hash      map[string][]storage.Row
	rightRows []storage.Row // insertion order; used by CROSS and RIGHT/FULL tail

	// probe pipeline
	leftIt   RowIter
	jobs     chan joinJob
	results  chan joinBatchResult
	done     chan struct{}
	pipeline sync.WaitGroup

	// ordered consumption
	nextSeq   int
	received  map[int]joinBatchResult
	probeDone bool
	curRows   []storage.Row
	curPos    int

	// tail rows (RIGHT/FULL unmatched right side), emitted after probe
	tail     []storage.Row
	tailPos  int
	tailDone bool

	crossProd bool

	matchedMu sync.Mutex
	matched   map[string]bool

	err      error
	errMu    sync.Mutex
	closeOne sync.Once
}

type joinJob struct {
	seq  int
	rows []storage.Row
}

type joinBatchResult struct {
	seq  int
	rows []storage.Row
	err  error
}

// Iterate builds the hash table from the right side (streaming, once), then
// starts the parallel probe pipeline over the left side.
func (n *JoinNode) Iterate() (RowIter, error) {
	normalize := n.NormalizeKey
	if normalize == nil {
		normalize = func(s string) string { return s }
	}
	it := &joinIter{
		node:      n,
		normalize: normalize,
		leftCol:   n.LeftColumn,
		rightCol:  n.RightColumn,
		received:  make(map[int]joinBatchResult),
		done:      make(chan struct{}),
	}

	// --- stage 1: build (stream right side once, never re-read) ---
	rightIt, err := n.Right.Iterate()
	if err != nil {
		return nil, err
	}
	if n.JoinType == "CROSS" {
		it.crossProd = true
		it.rightRows, err = Collect(rightIt)
		if err != nil {
			return nil, err
		}
	} else {
		it.hash = make(map[string][]storage.Row)
		for {
			row, rerr := rightIt.Next()
			if rerr == io.EOF {
				break
			}
			if rerr != nil {
				rightIt.Close()
				return nil, rerr
			}
			key := it.normalize(row[it.rightCol])
			it.hash[key] = append(it.hash[key], row)
			it.rightRows = append(it.rightRows, row)
		}
		rightIt.Close()
	}
	if n.JoinType == "RIGHT" || n.JoinType == "FULL" {
		it.matched = make(map[string]bool)
	}

	// --- stage 2: parallel probe over the left side ---
	leftIt, err := n.Left.Iterate()
	if err != nil {
		return nil, err
	}
	it.leftIt = leftIt
	it.jobs = make(chan joinJob, joinQueueDepth)
	it.results = make(chan joinBatchResult, joinQueueDepth)

	workers := runtime.GOMAXPROCS(0)
	if workers < 2 {
		workers = 2
	}
	// reader + workers + closer all report through pipeline
	it.pipeline.Add(workers + 1)
	for i := 0; i < workers; i++ {
		go it.probeWorker()
	}
	go func() {
		it.pipeline.Wait()
		close(it.results)
	}()
	go it.readBatches()

	return it, nil
}

func (n *JoinNode) Execute() ([]storage.Row, error) {
	return materialize(n)
}

// readBatches pulls probe rows from the child iterator and dispatches batches
// to the worker pool. Runs in its own goroutine.
func (it *joinIter) readBatches() {
	defer it.pipeline.Done()
	seq := 0
	var batch []storage.Row
	defer func() {
		if len(batch) > 0 {
			select {
			case it.jobs <- joinJob{seq: seq, rows: batch}:
			case <-it.done:
			}
		}
		close(it.jobs)
	}()
	for {
		row, err := it.leftIt.Next()
		if err != nil {
			if err != io.EOF {
				it.setErr(err)
			}
			return
		}
		select {
		case <-it.done:
			return
		default:
		}
		batch = append(batch, row)
		if len(batch) >= joinProbeBatchSize {
			select {
			case it.jobs <- joinJob{seq: seq, rows: batch}:
				seq++
				batch = nil
			case <-it.done:
				return
			}
		}
	}
}

// probeWorker consumes probe batches, joins them against the shared build
// hash, and publishes results tagged with the batch sequence number.
func (it *joinIter) probeWorker() {
	defer it.pipeline.Done()
	for j := range it.jobs {
		rows, err := it.joinBatch(j.rows)
		if err != nil {
			it.setErr(err)
			select {
			case it.results <- joinBatchResult{seq: j.seq, err: err}:
			case <-it.done:
			}
			continue
		}
		select {
		case it.results <- joinBatchResult{seq: j.seq, rows: rows}:
		case <-it.done:
		}
	}
}

// joinBatch joins one probe batch against the shared hash table.
func (it *joinIter) joinBatch(batch []storage.Row) ([]storage.Row, error) {
	n := it.node
	var out []storage.Row

	if it.crossProd {
		for _, lr := range batch {
			for _, rr := range it.rightRows {
				merged := make(storage.Row, len(lr)+len(rr))
				for k, v := range lr {
					merged[k] = v
				}
				for k, v := range rr {
					merged[k] = v
				}
				out = append(out, merged)
			}
		}
		return out, nil
	}

	for _, lr := range batch {
		key := it.normalize(lr[it.leftCol])
		if matched, ok := it.hash[key]; ok {
			it.recordMatched(key)
			if n.JoinType == "SEMI" {
				out = append(out, copyRow(lr))
				continue
			}
			for _, rr := range matched {
				merged := make(storage.Row, len(lr)+len(rr))
				for k, v := range lr {
					merged[k] = v
				}
				for k, v := range rr {
					if _, conflict := merged[k]; conflict && n.RightPrefix != "" {
						// same-named column on both sides: keep the left value
						// bare and store the right value qualified so
						// expressions like "alias.col" can still read it
						merged[n.RightPrefix+"."+k] = v
					} else {
						merged[k] = v
					}
				}
				out = append(out, merged)
			}
		} else if n.JoinType == "LEFT" || n.JoinType == "FULL" {
			out = append(out, copyRow(lr))
		}
	}
	return out, nil
}

func (it *joinIter) recordMatched(key string) {
	if it.matched == nil {
		return
	}
	it.matchedMu.Lock()
	it.matched[key] = true
	it.matchedMu.Unlock()
}

// Next returns the next joined row in serial (left-input) order.
func (it *joinIter) Next() (storage.Row, error) {
	for {
		// 1. drain the current batch
		if it.curPos < len(it.curRows) {
			row := it.curRows[it.curPos]
			it.curPos++
			return row, nil
		}
		// 2. take the next in-order batch if already received
		if res, ok := it.received[it.nextSeq]; ok {
			delete(it.received, it.nextSeq)
			it.nextSeq++
			if res.err != nil {
				return nil, res.err
			}
			it.curRows = res.rows
			it.curPos = 0
			continue
		}
		// 3. probe finished and no in-order batch left: emit tail rows
		if it.probeDone {
			if err := it.takeErr(); err != nil {
				return nil, err
			}
			if it.tailPos < len(it.tail) {
				row := it.tail[it.tailPos]
				it.tailPos++
				return row, nil
			}
			if !it.tailDone {
				it.materializeTail()
				it.tailDone = true
				continue
			}
			return nil, io.EOF
		}
		// 4. block for the next worker result
		select {
		case res, ok := <-it.results:
			if !ok {
				it.probeDone = true
				continue
			}
			if res.err != nil {
				return nil, res.err
			}
			it.received[res.seq] = res
		case <-it.done:
			return nil, io.EOF
		}
	}
}

// materializeTail computes unmatched right rows for RIGHT/FULL joins once
// every probe batch has been processed.
func (it *joinIter) materializeTail() {
	n := it.node
	if n.JoinType != "RIGHT" && n.JoinType != "FULL" {
		return
	}
	it.matchedMu.Lock()
	defer it.matchedMu.Unlock()
	for _, rr := range it.rightRows {
		key := it.normalize(rr[it.rightCol])
		if it.matched[key] {
			continue
		}
		it.tail = append(it.tail, copyRow(rr))
	}
}

func (it *joinIter) setErr(err error) {
	it.errMu.Lock()
	if it.err == nil {
		it.err = err
	}
	it.errMu.Unlock()
}

func (it *joinIter) takeErr() error {
	it.errMu.Lock()
	defer it.errMu.Unlock()
	return it.err
}

// Close shuts the probe pipeline down. Safe to call multiple times and safe
// to call while the pipeline is still running.
func (it *joinIter) Close() error {
	it.closeOne.Do(func() {
		close(it.done)
	})
	if it.leftIt != nil {
		it.leftIt.Close()
	}
	return nil
}
