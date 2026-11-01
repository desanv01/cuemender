//go:build reviewrepro

package diagnostics

import (
	"testing"
	"time"
)

func TestReviewR01MissingProgramUUIDIsUnknown(t *testing.T) {
	ev := baseEvidence(time.Now())
	ev.Observation.ProgramSceneUUID = ""
	hidden := false
	ev.Observation.VisualItemEnabled = &hidden
	result := EvaluateFor(ev, "video")
	if len(result.Findings) > 0 || result.Sufficient {
		t.Fatal("configured scene UUID was not observed, but visual finding was accepted")
	}
}
