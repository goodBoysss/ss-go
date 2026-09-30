// Package logging writes application logs to daily files in the server's local time.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DailyWriter serializes writes and switches files on the first entry of each day.
type DailyWriter struct {
	mu     sync.Mutex
	dir    string
	day    string
	file   *os.File
	now    func() time.Time
	closed bool
}

// NewDailyWriter creates the log directory and opens today's file in append mode.
func NewDailyWriter(dir string) (*DailyWriter, error) {
	w := &DailyWriter{dir: dir, now: time.Now}
	if err := w.rotate(); err != nil {
		return nil, err
	}
	return w, nil
}

// rotate opens the current day's file before closing the previous one; callers hold mu.
func (w *DailyWriter) rotate() error {
	day := w.now().Format("2006-01-02")
	if w.file != nil && w.day == day {
		return nil
	}
	if err := os.MkdirAll(w.dir, 0750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(filepath.Join(w.dir, day+".log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("open daily log: %w", err)
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	w.file, w.day = file, day
	return nil
}

// Write appends an entry to its day's log, falling back to stderr if disk writes fail.
func (w *DailyWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, os.ErrClosed
	}
	err := w.rotate()
	if err == nil {
		var n int
		n, err = w.file.Write(data)
		if err == nil && n == len(data) {
			return n, nil
		}
		if err == nil {
			err = io.ErrShortWrite
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "ssgo: log file write failed: %v\n", err)
	return os.Stderr.Write(data)
}

// Close releases the log file and prevents further writes; repeated calls are safe.
func (w *DailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return nil
	}
	w.closed = true
	return w.file.Close()
}
