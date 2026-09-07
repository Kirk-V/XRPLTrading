// Package storage contains local persistence primitives. This event journal
// is intentionally limited to normalized, non-secret application events.
package storage

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Event struct {
	Sequence    uint64          `json:"sequence"`
	Type        string          `json:"type"`
	RecordedAt  time.Time       `json:"recorded_at"`
	LedgerIndex uint32          `json:"ledger_index,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

func NewEvent(eventType string, ledgerIndex uint32, payload any) (Event, error) {
	if eventType == "" {
		return Event{}, errors.New("event type is required")
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("encode event payload: %w", err)
	}
	return Event{
		Type:        eventType,
		RecordedAt:  time.Now().UTC(),
		LedgerIndex: ledgerIndex,
		Payload:     encoded,
	}, nil
}

type Journal interface {
	Append(context.Context, Event) (Event, error)
	Replay(context.Context, uint64, func(Event) error) error
	Close() error
}

// FileJournal is an append-only JSON Lines journal. Each append is flushed to
// disk before returning, making the sequence a usable local recovery cursor.
type FileJournal struct {
	mu     sync.Mutex
	file   *os.File
	path   string
	nextID uint64
}

func OpenFileJournal(path string) (*FileJournal, error) {
	if path == "" {
		return nil, errors.New("journal path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create journal directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open event journal: %w", err)
	}
	j := &FileJournal{file: file, path: path, nextID: 1}
	if err := j.scan(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return j, nil
}

func (j *FileJournal) Append(ctx context.Context, event Event) (Event, error) {
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	if event.Type == "" || len(event.Payload) == 0 {
		return Event{}, errors.New("event type and payload are required")
	}
	if !json.Valid(event.Payload) {
		return Event{}, errors.New("event payload must be valid JSON")
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return Event{}, errors.New("event journal is closed")
	}
	if err := ctx.Err(); err != nil {
		return Event{}, err
	}
	event.Sequence = j.nextID
	if event.RecordedAt.IsZero() {
		event.RecordedAt = time.Now().UTC()
	}
	line, err := json.Marshal(event)
	if err != nil {
		return Event{}, fmt.Errorf("encode journal event: %w", err)
	}
	if _, err := j.file.Write(append(line, '\n')); err != nil {
		return Event{}, fmt.Errorf("append journal event: %w", err)
	}
	if err := j.file.Sync(); err != nil {
		return Event{}, fmt.Errorf("flush journal event: %w", err)
	}
	j.nextID++
	return event, nil
}

func (j *FileJournal) Replay(ctx context.Context, after uint64, visit func(Event) error) error {
	if visit == nil {
		return errors.New("replay callback is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return errors.New("event journal is closed")
	}
	if _, err := j.file.Seek(0, 0); err != nil {
		return fmt.Errorf("rewind event journal: %w", err)
	}
	scanner := bufio.NewScanner(j.file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("decode event journal %q: %w", j.path, err)
		}
		if event.Sequence <= after {
			continue
		}
		if err := visit(event); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read event journal: %w", err)
	}
	return nil
}

func (j *FileJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.file == nil {
		return nil
	}
	err := j.file.Close()
	j.file = nil
	return err
}

func (j *FileJournal) scan() error {
	if _, err := j.file.Seek(0, 0); err != nil {
		return fmt.Errorf("rewind event journal: %w", err)
	}
	scanner := bufio.NewScanner(j.file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	var last uint64
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return fmt.Errorf("decode event journal %q: %w", j.path, err)
		}
		if event.Sequence == 0 || event.Sequence <= last {
			return fmt.Errorf("invalid event sequence %d in %q", event.Sequence, j.path)
		}
		last = event.Sequence
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan event journal: %w", err)
	}
	j.nextID = last + 1
	_, err := j.file.Seek(0, 2)
	return err
}
