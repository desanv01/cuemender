//go:build reviewrepro

package incidents

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
	"github.com/desanv01/cuemender/internal/store"
)

func TestReviewR05ConditionalInterruptionPreservesNewerState(t *testing.T) {
	for _, newerStage := range []string{domain.StageInvestigating, domain.StageDismissed} {
		t.Run(newerStage, func(t *testing.T) {
			_, _, st := coordinatorFixture(t)
			ctx := context.Background()
			oldTime := time.Now().UTC().Add(-time.Minute)
			inc, err := st.CreateManualIncident(ctx, "review-session", "audio", "review-cas", "CM-CAS", oldTime)
			if err != nil {
				t.Fatal(err)
			}
			inc.Stage = domain.StageInvestigating
			if err := st.SaveIncident(ctx, inc); err != nil {
				t.Fatal(err)
			}
			// Deterministic interleaving: cleanup observed oldTime, then dismissal/recheck wins.
			inc.Stage = newerStage
			inc.UpdatedAt = oldTime.Add(time.Second)
			if err := st.SaveIncident(ctx, inc); err != nil {
				t.Fatal(err)
			}
			_, changed, err := st.InterruptIncident(ctx, inc.ID, oldTime, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if changed {
				t.Fatal("old cleanup overwrote newer version")
			}
			got, err := st.GetIncident(ctx, inc.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Stage != newerStage || !got.UpdatedAt.Equal(inc.UpdatedAt) {
				t.Fatal("newer durable outcome changed")
			}
		})
	}
}

func TestReviewR05CanceledJobPersistsInterruption(t *testing.T) {
	c, _, st := coordinatorFixture(t)
	entered := make(chan struct{})
	c.read = func(ctx context.Context, _ domain.Settings) (*domain.OBSDiscovery, *domain.OBSObservation, error) {
		close(entered)
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	inc, err := c.Check(context.Background(), "audio", 1, "")
	if err != nil {
		t.Fatal(err)
	}
	<-entered
	c.mu.Lock()
	c.jobs["audio"].cancel()
	c.mu.Unlock()
	c.wg.Wait()
	durable, err := st.GetIncident(context.Background(), inc.ID)
	if err != nil {
		t.Fatal(err)
	}
	if durable.Stage != domain.StageInterrupted {
		t.Fatalf("canceled job exited but durable stage=%s, want interrupted", durable.Stage)
	}
	cached, _ := c.Find(inc.ID)
	if cached.Stage != durable.Stage {
		t.Fatal("cache differs from durable interruption")
	}
}

func TestReviewR05CloseJoinsDatabaseBlockedStart(t *testing.T) {
	_, settings, _ := coordinatorFixture(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "blocked.db")
	st, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	entered := make(chan struct{})
	c, err := NewChecked(Options{Store: st, Settings: func() domain.Settings { return *settings },
		Status: func() Readiness {
			close(entered)
			return Readiness{TwitchConnected: true, TwitchGeneration: 1, OBSConnected: true, ResolutionReady: true}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
		t.Fatal(err)
	}
	defer conn.ExecContext(ctx, "ROLLBACK")
	started := make(chan error, 1)
	go func() { _, err := c.Start(ctx, 1); started <- err }()
	<-entered
	select {
	case err := <-started:
		t.Fatalf("precondition: Start did not block on database: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	closed := make(chan struct{})
	go func() { c.Close(); close(closed) }()
	select {
	case <-closed:
		// Early return is valid only if Close canceled and joined the blocked call.
		select {
		case err := <-started:
			if err == nil {
				t.Error("blocked start succeeded before lock release")
			}
		default:
			t.Error("Close returned without joining blocked Start")
		}
	case <-time.After(100 * time.Millisecond):
		// Joining an admitted bounded operation is also allowed.
	}
	if _, err := conn.ExecContext(ctx, "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not complete after database released")
	}
	select {
	case <-started:
	default:
	}
}
