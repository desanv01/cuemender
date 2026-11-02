package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/desanv01/cuemender/internal/domain"
)

func (s *Store) LoadRepairs(ctx context.Context) ([]domain.Proposal, []domain.Action, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT proposal_json FROM proposals`)
	if err != nil {
		return nil, nil, err
	}
	var proposals []domain.Proposal
	for rows.Next() {
		var raw string
		var p domain.Proposal
		if err = rows.Scan(&raw); err == nil {
			err = json.Unmarshal([]byte(raw), &p)
		}
		if err != nil {
			rows.Close()
			return nil, nil, err
		}
		proposals = append(proposals, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, nil, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT action_json FROM actions`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var actions []domain.Action
	for rows.Next() {
		var raw string
		var a domain.Action
		if err = rows.Scan(&raw); err == nil {
			err = json.Unmarshal([]byte(raw), &a)
		}
		if err != nil {
			return nil, nil, err
		}
		actions = append(actions, a)
	}
	return proposals, actions, rows.Err()
}

func putProposal(ctx context.Context, tx *sql.Tx, p domain.Proposal) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO proposals(id,incident_id,status,proposal_json) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,proposal_json=excluded.proposal_json`, p.ID, p.IncidentID, p.Status, string(raw))
	return err
}

func putAction(ctx context.Context, tx *sql.Tx, a domain.Action) error {
	raw, err := json.Marshal(a)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO actions(id,proposal_id,incident_id,status,action_json) VALUES(?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET status=excluded.status,action_json=excluded.action_json`, a.ID, a.ProposalID, a.IncidentID, a.Status, string(raw))
	return err
}

// SaveRepair commits the related proposal, action and incident as one change.
// intent additionally compares the durable settings authority before any write.
func (s *Store) SaveRepair(ctx context.Context, p *domain.Proposal, a *domain.Action, inc *domain.Incident, intent bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if intent {
		var revision int64
		if err = tx.QueryRowContext(ctx, `SELECT revision FROM app_settings WHERE id=1`).Scan(&revision); err != nil {
			return err
		}
		if a == nil || revision != a.Binding.ConfigRevision {
			return domain.ErrRevisionConflict
		}
		var status string
		if err = tx.QueryRowContext(ctx, `SELECT status FROM actions WHERE id=?`, a.ID).Scan(&status); err != nil {
			return err
		}
		if status != domain.ActionReserved {
			return domain.ErrActionConflict
		}
	}
	if p != nil {
		if err = putProposal(ctx, tx, *p); err != nil {
			return err
		}
	}
	if a != nil {
		if err = putAction(ctx, tx, *a); err != nil {
			return err
		}
	}
	if inc != nil {
		if err = saveIncidentTx(ctx, tx, *inc); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// RecoverRepairs never replays: reservations without intent are invalidated;
// unfinished intents are conservatively unknown, even before write_sent exists.
func (s *Store) RecoverRepairs(ctx context.Context, now time.Time) error {
	proposals, actions, err := s.LoadRepairs(ctx)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range proposals {
		if p.Status == domain.ProposalProposed || p.Status == domain.ProposalReserved {
			p.Status = domain.ProposalInvalidated
			if err = putProposal(ctx, tx, p); err != nil {
				return err
			}
		}
	}
	for _, a := range actions {
		if !domain.ActionPending(a.Status) {
			continue
		}
		if a.IntentAt != nil || a.Status == "write_sent" || a.Status == domain.TrackRepairUnknown {
			a.Status = domain.TrackRepairUnknown
			a.PossiblyTransmitted = true
			a.DetailCode = "restart_outcome_unknown"
		} else {
			a.Status = domain.TrackRepairInvalidated
			a.DetailCode = "restart_before_intent"
		}
		a.UpdatedAt = now.UTC()
		if err = putAction(ctx, tx, a); err != nil {
			return err
		}
		if a.Status == domain.TrackRepairUnknown {
			inc, readErr := incidentFromRow(func(dest ...any) error {
				return tx.QueryRowContext(ctx, incidentSelect+` WHERE id=?`, a.IncidentID).Scan(dest...)
			})
			if readErr != nil {
				return readErr
			}
			inc.Stage = domain.StageActionUnknown
			inc.LocalVerification = domain.LocalUnknown
			inc.RepairAvailable = false
			inc.UpdatedAt = now.UTC()
			if err = saveIncidentTx(ctx, tx, inc); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
