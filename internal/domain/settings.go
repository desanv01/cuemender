package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ErrRevisionConflict is returned when a settings update is based on an
// outdated revision. It is a safe sentinel for HTTP 409 handling.
var ErrRevisionConflict = errors.New("settings revision conflict")

const (
	ModeLive      = "live"
	ModeRehearsal = "rehearsal"

	TrackMin = 1
	TrackMax = 6
)

// InputRef identifies an OBS input without assuming an OBS SDK type. P01
// validates structure only; P02 resolves it against a live OBS instance.
type InputRef struct {
	Name    string `json:"name"`
	UUID    string `json:"uuid,omitempty"`
	Context string `json:"context,omitempty"`
}

// VisualExpectation describes one directly configured scene item. Presence of
// the value means the expectation is enabled.
type VisualExpectation struct {
	Scene       string   `json:"scene"`
	SceneUUID   string   `json:"scene_uuid,omitempty"`
	SceneItemID string   `json:"scene_item_id"`
	Source      InputRef `json:"source"`
}

// Expectations is the single versioned record stored in SQLite. A nil value
// means "not configured" rather than an invented default.
type Expectations struct {
	Microphone       *InputRef          `json:"microphone"`
	StreamTrack      *int               `json:"stream_track"`
	TrackConfirmedAt *time.Time         `json:"track_confirmed_at"`
	Visual           *VisualExpectation `json:"visual"`
}

// Settings is the persisted expectations record plus its revision.
type Settings struct {
	Revision int64 `json:"revision"`
	Expectations
}

// Validate enforces structural validity only. It does not claim that any
// referenced OBS input, scene or scene item exists.
func (e Expectations) Validate() error {
	if e.Microphone != nil {
		if err := validateInputRef("microphone", *e.Microphone); err != nil {
			return err
		}
	}
	if e.StreamTrack != nil {
		if *e.StreamTrack < TrackMin || *e.StreamTrack > TrackMax {
			return fmt.Errorf("stream_track must be between %d and %d", TrackMin, TrackMax)
		}
	} else if e.TrackConfirmedAt != nil {
		return errors.New("track_confirmed_at requires stream_track")
	}
	if e.Visual != nil {
		if err := validateText("visual.scene", e.Visual.Scene, true, 256); err != nil {
			return err
		}
		if e.Visual.SceneUUID != "" {
			if err := validateText("visual.scene_uuid", e.Visual.SceneUUID, false, 256); err != nil {
				return err
			}
		}
		if err := validateText("visual.scene_item_id", e.Visual.SceneItemID, true, 256); err != nil {
			return err
		}
		if err := validateInputRef("visual.source", e.Visual.Source); err != nil {
			return err
		}
	}
	return nil
}

func validateInputRef(field string, ref InputRef) error {
	if err := validateText(field+".name", ref.Name, true, 256); err != nil {
		return err
	}
	if ref.UUID != "" {
		if err := validateText(field+".uuid", ref.UUID, false, 256); err != nil {
			return err
		}
	}
	if ref.Context != "" {
		if err := validateText(field+".context", ref.Context, false, 256); err != nil {
			return err
		}
	}
	return nil
}

func validateText(field, value string, required bool, maxRunes int) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", field)
		}
		return nil
	}
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("%s must not have leading or trailing whitespace", field)
	}
	if utf8.RuneCountInString(value) > maxRunes {
		return fmt.Errorf("%s is too long", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s contains a control character", field)
		}
	}
	return nil
}
