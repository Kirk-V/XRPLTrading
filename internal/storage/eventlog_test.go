package storage

import (
	"context"
	"reflect"
	"testing"
)

func TestFileJournalPersistsAndReopens(t *testing.T) {
	path := t.TempDir() + "/events/events.jsonl"
	first, err := OpenFileJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	event, err := NewEvent("LedgerClosed", 42, map[string]any{"hash": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := first.Append(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Sequence != 1 {
		t.Fatalf("expected first sequence, got %d", stored.Sequence)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := OpenFileJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	var replayed []Event
	if err := second.Replay(context.Background(), 0, func(event Event) error {
		replayed = append(replayed, event)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(replayed) != 1 || replayed[0].Sequence != 1 || replayed[0].Type != "LedgerClosed" {
		t.Fatalf("unexpected replay: %+v", replayed)
	}

	next, err := second.Append(context.Background(), event)
	if err != nil {
		t.Fatal(err)
	}
	if next.Sequence != 2 {
		t.Fatalf("expected sequence to continue at 2, got %d", next.Sequence)
	}
	if !reflect.DeepEqual(replayed[0].Payload, event.Payload) {
		t.Fatal("journal changed event payload")
	}
}

func TestFileJournalRejectsInvalidPayload(t *testing.T) {
	journal, err := OpenFileJournal(t.TempDir() + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	_, err = journal.Append(context.Background(), Event{Type: "BookUpdated", Payload: []byte("not-json")})
	if err == nil {
		t.Fatal("expected invalid JSON payload error")
	}
}
