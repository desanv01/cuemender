// Promoted from Codex's P05-R01 review set; assertions preserved.
package store

import (
	"context"
	"database/sql"
	"errors"
	"github.com/desanv01/cuemender/internal/domain"
	"path/filepath"
	"testing"
	"time"
)

func TestReviewP05LegacyModeRejectedBeforeMigration(t *testing.T) {
	for _, version := range []int{1, 3} {
		t.Run(string(rune('0'+version)), func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "legacy.db")
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range migrations {
				if m.version > version {
					break
				}
				if _, err = db.Exec(m.sql); err != nil {
					t.Fatal(err)
				}
				if _, err = db.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?,?)", m.version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
					t.Fatal(err)
				}
			}
			db.Close()
			st, err := OpenMode(ctx, path, domain.ModeRehearsal)
			if st != nil {
				st.Close()
			}
			if !errors.Is(err, domain.ErrModeConflict) {
				t.Errorf("legacy live schema %d admitted as rehearsal: %v", version, err)
			}
			db, err = sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			var actual int
			if err = db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&actual); err != nil {
				t.Fatal(err)
			}
			if actual != version {
				t.Errorf("rejected mode changed schema: before=%d after=%d", version, actual)
			}
		})
	}
}

func TestReviewP05ClosureInvalidatesSerializedProposalAtomically(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	inc, err := st.CreateManualIncident(ctx, "session", "audio", "closure-proposal", "CM-P05", now)
	if err != nil {
		t.Fatal(err)
	}
	inc.Stage = domain.StageAwaitingApproval
	p := domain.Proposal{ID: "proposal-close", IncidentID: inc.ID, Status: domain.ProposalProposed}
	if err = st.SaveRepair(ctx, &p, nil, &inc, false); err != nil {
		t.Fatal(err)
	}
	if _, err = st.CloseIncident(ctx, inc.ID, "unresolved", "session", now); err != nil {
		t.Fatal(err)
	}
	ps, _, err := st.LoadRepairs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Status != domain.ProposalInvalidated {
		t.Fatalf("LoadRepairs still exposes executable proposal after transactional close: %+v", ps)
	}
}
