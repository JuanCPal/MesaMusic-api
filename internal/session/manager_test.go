package session

import (
	"testing"

	"musica-colaborativa-api/internal/models"
)

func TestManagerCreateCreatesIndependentQueues(t *testing.T) {
	mgr := NewManager([]string{"backup-1", "backup-2"})

	first := mgr.Create("Cumpleanos de Ana")
	second := mgr.Create("After Office")

	if first.ID == second.ID {
		t.Fatalf("expected unique session IDs, got %q", first.ID)
	}

	if first.Queue == second.Queue {
		t.Fatal("expected each session to own a different queue manager")
	}

	first.Queue.Add(models.Song{VideoID: "abc123", Title: "Tema", DurationSeconds: 180})

	firstState := first.Queue.State()
	secondState := second.Queue.State()

	if firstState.NowPlaying == nil {
		t.Fatal("expected first session queue to start playback after adding a song")
	}

	if secondState.NowPlaying != nil {
		t.Fatal("expected second session queue to remain idle")
	}

	if len(secondState.Queue) != 0 {
		t.Fatalf("expected second session queue to remain empty, got %d items", len(secondState.Queue))
	}
}

func TestManagerCallsOnSessionCreated(t *testing.T) {
	mgr := NewManager(nil)
	var gotID string

	mgr.SetOnSessionCreated(func(s *Session) {
		gotID = s.ID
	})

	session := mgr.Create("Sesion con hook")
	if gotID != session.ID {
		t.Fatalf("expected hook to receive session ID %q, got %q", session.ID, gotID)
	}
}

func TestManagerEndMarksSessionAsEndedWithoutDeletingIt(t *testing.T) {
	mgr := NewManager(nil)
	session := mgr.Create("Sesion de prueba")

	if ok := mgr.End(session.ID); !ok {
		t.Fatal("expected End to succeed for an existing session")
	}

	stored, ok := mgr.Get(session.ID)
	if !ok {
		t.Fatal("expected ended session to remain stored")
	}

	if stored.Status != StatusEnded {
		t.Fatalf("expected session status %q, got %q", StatusEnded, stored.Status)
	}
}

func TestManagerEndReturnsFalseForUnknownSession(t *testing.T) {
	mgr := NewManager(nil)

	if ok := mgr.End("missing"); ok {
		t.Fatal("expected End to return false for an unknown session")
	}
}
