// Promoted from Codex's P03-R03 review set; assertions preserved.
package app

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/store"
)

func TestReviewR02StartupRecoveryErrorPropagates(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, databaseFileName)
	st, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.CreateManualIncident(ctx, "old-session", "audio", "old-incident", "CM-OLD", time.Now()); err != nil {
		st.Close()
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TRIGGER review_recovery_failure BEFORE UPDATE ON incidents BEGIN SELECT RAISE(ABORT,'review recovery fault'); END`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	app, err := New(ctx, Options{DataDir: dir, ListenAddress: fmt.Sprintf("127.0.0.1:%d", port), NoBrowser: true, SessionOnlySecrets: true, Stdout: io.Discard, Stderr: io.Discard})
	if app != nil {
		app.Close()
	}
	if err == nil {
		t.Fatal("application startup succeeded after injected recovery transaction failure")
	}
}
