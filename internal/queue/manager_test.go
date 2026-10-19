package queue

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"musica-colaborativa-api/internal/models"
)

func TestRemoveItemRemovesPendingItem(t *testing.T) {
	mgr := NewManager(nil)

	nowPlaying := mgr.Add(models.Song{VideoID: "np-1", Title: "Now Playing", DurationSeconds: 180}, "Ana")
	toRemove := mgr.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")
	remaining := mgr.Add(models.Song{VideoID: "q-2", Title: "Q2", DurationSeconds: 150}, "Ana")

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

	_ = mgr.Add(models.Song{VideoID: "np-1", Title: "Now Playing", DurationSeconds: 180}, "Ana")
	pending := mgr.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")

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

func TestRemoveItemReturnsErrItemNotFoundOnEmptyQueue(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.RemoveItem("missing-item")
	if !errors.Is(err, ErrItemNotFound) {
		t.Fatalf("expected ErrItemNotFound, got %v", err)
	}

	state := mgr.State()
	if state.NowPlaying != nil {
		t.Fatal("expected nowPlaying to remain nil")
	}

	if len(state.Queue) != 0 {
		t.Fatalf("expected empty queue, got %d items", len(state.Queue))
	}
}

func TestSkipCurrentWithPendingAdvancesToNextSong(t *testing.T) {
	mgr := NewManager(nil)

	_ = mgr.Add(models.Song{VideoID: "np-1", Title: "Now Playing", DurationSeconds: 180}, "Ana")
	next := mgr.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")

	mgr.SkipCurrent()

	state := mgr.State()
	if state.NowPlaying == nil {
		t.Fatal("expected nowPlaying after skip")
	}

	if state.NowPlaying.ID != next.ID {
		t.Fatalf("expected nowPlaying ID %q, got %q", next.ID, state.NowPlaying.ID)
	}

	if len(state.Queue) != 0 {
		t.Fatalf("expected pending queue to be empty after skip, got %d items", len(state.Queue))
	}
}

func TestSkipCurrentWithEmptyQueueFallsBackToBackup(t *testing.T) {
	mgr := NewManager([]string{"backup-1"})

	mgr.SkipCurrent()

	state := mgr.State()
	if state.NowPlaying == nil {
		t.Fatal("expected backup song to start")
	}

	if !state.NowPlaying.IsBackup {
		t.Fatal("expected nowPlaying to be backup item")
	}

	if state.NowPlaying.Song.VideoID != "backup-1" {
		t.Fatalf("expected backup video ID %q, got %q", "backup-1", state.NowPlaying.Song.VideoID)
	}
}

func TestSkipCurrentWithoutBackupLeavesNowPlayingNil(t *testing.T) {
	mgr := NewManager(nil)

	mgr.SkipCurrent()

	state := mgr.State()
	if state.NowPlaying != nil {
		t.Fatal("expected nowPlaying to remain nil")
	}

	if len(state.Queue) != 0 {
		t.Fatalf("expected empty queue, got %d items", len(state.Queue))
	}
}

func TestRemoveItemTriggersNotifyOnSuccess(t *testing.T) {
	mgr := NewManager(nil)

	_ = mgr.Add(models.Song{VideoID: "np-1", Title: "Now Playing", DurationSeconds: 180}, "Ana")
	toRemove := mgr.Add(models.Song{VideoID: "q-1", Title: "Q1", DurationSeconds: 120}, "Ana")

	var notifyCount int32
	mgr.SetOnChange(func(_ models.QueueState) {
		atomic.AddInt32(&notifyCount, 1)
	})

	if err := mgr.RemoveItem(toRemove.ID); err != nil {
		t.Fatalf("expected RemoveItem to succeed, got %v", err)
	}

	if got := atomic.LoadInt32(&notifyCount); got != 1 {
		t.Fatalf("expected notify to be called once, got %d", got)
	}
}

func TestSkipCurrentTriggersNotify(t *testing.T) {
	mgr := NewManager([]string{"backup-1"})

	var notifyCount int32
	mgr.SetOnChange(func(_ models.QueueState) {
		atomic.AddInt32(&notifyCount, 1)
	})

	mgr.SkipCurrent()

	if got := atomic.LoadInt32(&notifyCount); got != 1 {
		t.Fatalf("expected notify to be called once, got %d", got)
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	mgr := NewManager([]string{"backup-1", "backup-2"})

	var wg sync.WaitGroup
	workers := 32
	iterations := 20
	errCh := make(chan error, workers*iterations)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				item := mgr.Add(models.Song{
					VideoID:         fmt.Sprintf("video-%d-%d", workerID, i),
					Title:           "Concurrent",
					DurationSeconds: 120,
				}, "Ana")

				err := mgr.RemoveItem(item.ID)
				if err != nil && !errors.Is(err, ErrItemNotFound) {
					errCh <- fmt.Errorf("unexpected RemoveItem error: %w", err)
					return
				}

				mgr.SkipCurrent()
				_ = mgr.State()
			}
		}(w)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatal(err)
	}
}

func TestAddStoresRequestedBy(t *testing.T) {
mgr := NewManager(nil)

item := mgr.Add(models.Song{VideoID: "v1"}, "Ana")

if item.RequestedBy != "Ana" {
t.Fatalf("expected RequestedBy %q, got %q", "Ana", item.RequestedBy)
}
}

func TestBackupItemIsRequestedByLaCasa(t *testing.T) {
mgr := NewManager([]string{"backup-1"})

mgr.Ended()

np := mgr.State().NowPlaying
if np == nil || !np.IsBackup {
t.Fatal("expected a backup item to be playing")
}
if np.RequestedBy != "la casa" {
t.Fatalf("expected RequestedBy %q, got %q", "la casa", np.RequestedBy)
}
}
