package storage

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/vovakirdan/durak/internal/app"
)

var (
	// ErrInvalidEventLog means the JSONL event log path or contents are invalid.
	ErrInvalidEventLog = errors.New("invalid event log")
	// ErrCorruptEventLog means a JSONL event log contains an unreadable event line.
	ErrCorruptEventLog = errors.New("corrupt event log")
)

// JSONLEventStore appends stable event envelopes to a local JSONL file.
type JSONLEventStore struct {
	path string
	mu   sync.Mutex
}

// NewJSONLEventStore creates a JSONL-backed event store.
func NewJSONLEventStore(path string) (*JSONLEventStore, error) {
	if path == "" {
		return nil, fmt.Errorf("%w: path is empty", ErrInvalidEventLog)
	}
	return &JSONLEventStore{path: path}, nil
}

// Path returns the configured log file path.
func (s *JSONLEventStore) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// AppendEvents appends a validated batch of events as JSONL.
func (s *JSONLEventStore) AppendEvents(ctx context.Context, events []app.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.path == "" {
		return fmt.Errorf("%w: store path is empty", ErrInvalidEventLog)
	}
	if len(events) == 0 {
		return nil
	}

	var buffer bytes.Buffer
	for i := range events {
		data, err := app.MarshalEventJSON(&events[i])
		if err != nil {
			return err
		}
		buffer.Write(data)
		buffer.WriteByte('\n')
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ensureParentDir(s.path); err != nil {
		return err
	}
	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("%w: open %q: %w", ErrInvalidEventLog, s.path, err)
	}
	written, err := file.Write(buffer.Bytes())
	if err != nil {
		closeAfterFailure(file)
		return fmt.Errorf("%w: write %q: %w", ErrInvalidEventLog, s.path, err)
	}
	if written != buffer.Len() {
		closeAfterFailure(file)
		return fmt.Errorf("%w: short write %q: wrote %d of %d bytes", ErrInvalidEventLog, s.path, written, buffer.Len())
	}
	if err := file.Sync(); err != nil {
		closeAfterFailure(file)
		return fmt.Errorf("%w: sync %q: %w", ErrInvalidEventLog, s.path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("%w: close %q: %w", ErrInvalidEventLog, s.path, err)
	}
	return nil
}

// Events reads all stored events in file order.
func (s *JSONLEventStore) Events(ctx context.Context) ([]app.Event, error) {
	return s.readEvents(ctx, "")
}

// EventsForMatch reads stored events for one match stream in file order.
func (s *JSONLEventStore) EventsForMatch(ctx context.Context, matchID app.MatchID) ([]app.Event, error) {
	return s.readEvents(ctx, matchID)
}

func (s *JSONLEventStore) readEvents(ctx context.Context, matchID app.MatchID) ([]app.Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.path == "" {
		return nil, fmt.Errorf("%w: store path is empty", ErrInvalidEventLog)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: open %q: %w", ErrInvalidEventLog, s.path, err)
	}

	events, readErr := readEventsFrom(ctx, file, matchID)
	if err := file.Close(); err != nil {
		if readErr != nil {
			return nil, readErr
		}
		return nil, fmt.Errorf("%w: close %q: %w", ErrInvalidEventLog, s.path, err)
	}
	return events, readErr
}

func readEventsFrom(ctx context.Context, reader io.Reader, matchID app.MatchID) ([]app.Event, error) {
	var events []app.Event
	scanner := bufio.NewScanner(reader)
	for line := 1; scanner.Scan(); line++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data := bytes.TrimSpace(scanner.Bytes())
		if len(data) == 0 {
			continue
		}
		event, err := app.UnmarshalEventJSON(data)
		if err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrCorruptEventLog, line, err)
		}
		if matchID == "" || event.MatchID == matchID {
			events = append(events, event)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%w: scan: %w", ErrInvalidEventLog, err)
	}
	return events, nil
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: create dir %q: %w", ErrInvalidEventLog, dir, err)
	}
	return nil
}

func closeAfterFailure(file *os.File) {
	if err := file.Close(); err != nil {
		return
	}
}
