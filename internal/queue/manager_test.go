package queue

import (
	"errors"
	"testing"

	"musica-colaborativa-api/internal/models"
)

func TestRemoveItemRemovesPendingItem(t *testing.T) {
	mgr := NewManager(nil)

	nowPlaying := mgr.Add(models.Song{VideoID: "np-1", Title: "Now Playing", DurationSeconds: 180})
	toRemove := mgr.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120})
	remaining := mgr.Add(models.Song{VideoID: "q-2", Title: "Q2", DurationSeconds: 150})

	if err := mgr.RemoveItem(toRemove.ID); err != nil {
		t.Fatalf("expected RemoveItem to succeed, got error: %v", err)
	}

	state := mgr.State()
	if state.NowPlaying == nil {
		t.Fatal("expected nowPlaying to remain set")
	}

	if state.NowPlaying.ID != nowPlaying.ID {
		t.Fatalf("expected nowPlaying ID %q, got %q", nowPlaying.ID, state.NowPlaying.ID)
	}

	if len(state.Queue) != 1 {
		t.Fatalf("expected 1 pending item after removal, got %d", len(state.Queue))
	}

	if state.Queue[0].ID != remaining.ID {
		t.Fatalf("expected remaining pending item ID %q, got %q", remaining.ID, state.Queue[0].ID)
	}
}

func TestRemoveItemReturnsErrItemNotFound(t *testing.T) {
	mgr := NewManager(nil)

	_ = mgr.Add(models.Song{VideoID: "np-1", Title: "Now Playing", DurationSeconds: 180})
	pending := mgr.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120})

	err := mgr.RemoveItem("missing-item")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}

	state := mgr.State()
	if state.NowPlaying == nil {
		t.Fatal("expected nowPlaying to remain set")
	}

	if len(state.Queue) != 1 {
		t.Fatalf("expected pending queue to stay unchanged, got %d items", len(state.Queue))
	}

	if state.Queue[0].ID != pending.ID {
		t.Fatalf("expected pending item ID %q, got %q", pending.ID, state.Queue[0].ID)
	}
}
