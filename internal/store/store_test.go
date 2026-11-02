package store

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/desanv01/cuemender/internal/domain"
)

func testSettings(track int, mic string) domain.Expectations {
	confirmed := time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)
	return domain.Expectations{
		Microphone:       &domain.InputRef{Name: mic, UUID: "uuid-1", Context: "scene-collection"},
		StreamTrack:      &track,
		TrackConfirmedAt: &confirmed,
		Visual: &domain.VisualExpectation{
			Scene:       "Main",
			SceneItemID: "42",
			Source:      domain.InputRef{Name: "Overlay", UUID: "uuid-2"},
		},
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "space ü", "livecheck.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	st, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestOpenMigratesAndPRAGMAs(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	version, err := st.SchemaVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("schema version = %d, want 2", version)
	}
	var foreignKeys int
	if err := st.db.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}
	var busy int
	if err := st.db.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busy); err != nil {
		t.Fatal(err)
	}
	if busy != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busy)
	}
}

func TestSettingsPersistAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "livecheck.db")
	ctx := context.Background()
	st, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := st.SaveSettings(ctx, 0, testSettings(2, "Mic/Aux"))
	if err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}
	if saved.Revision != 1 {
		t.Fatalf("revision = %d, want 1", saved.Revision)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()
	loaded, err := reopened.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 1 || loaded.Microphone == nil || loaded.Microphone.Name != "Mic/Aux" {
		t.Fatalf("unexpected loaded settings: %+v", loaded)
	}
	if loaded.StreamTrack == nil || *loaded.StreamTrack != 2 {
		t.Fatalf("track did not survive: %+v", loaded.StreamTrack)
	}
	if loaded.Visual == nil || loaded.Visual.SceneItemID != "42" {
		t.Fatalf("visual did not survive: %+v", loaded.Visual)
	}
}

func TestRevisionConflict(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	if _, err := st.SaveSettings(ctx, 0, domain.Expectations{}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SaveSettings(ctx, 0, domain.Expectations{}); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("second stale save error = %v, want ErrRevisionConflict", err)
	}
	loaded, err := st.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 1 {
		t.Fatalf("revision = %d, want 1 after conflict", loaded.Revision)
	}
}

func TestInvalidSettingsRejectedWithoutWrite(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	badTrack := 7
	if _, err := st.SaveSettings(ctx, 0, domain.Expectations{StreamTrack: &badTrack}); err == nil {
		t.Fatal("expected invalid track to be rejected")
	}
	loaded, err := st.LoadSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Revision != 0 {
		t.Fatalf("revision = %d, want 0 after rejected write", loaded.Revision)
	}
}

func TestMigrationsRepeatSafely(t *testing.T) {
	path := filepath.Join(t.TempDir(), "livecheck.db")
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		st, err := Open(ctx, path)
		if err != nil {
			t.Fatalf("open %d: %v", i, err)
		}
		version, err := st.SchemaVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if version != 2 {
			t.Fatalf("version = %d, want 2", version)
		}
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "livecheck.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (99, '2026-11-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(context.Background(), path); !errors.Is(err, ErrSchemaNewer) {
		t.Fatalf("Open error = %v, want ErrSchemaNewer", err)
	}
}
