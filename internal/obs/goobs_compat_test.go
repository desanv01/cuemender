package obs

import (
	"testing"

	"github.com/andreykaipov/goobs/api/requests/general"
	"github.com/andreykaipov/goobs/api/requests/inputs"
	"github.com/andreykaipov/goobs/api/requests/sceneitems"
	"github.com/andreykaipov/goobs/api/requests/scenes"
	"github.com/andreykaipov/goobs/api/requests/stream"
	"github.com/andreykaipov/goobs/api/typedefs"
)

// This test compiles the actual pinned SDK method signatures and response
// fields used by the production adapter. It performs no network I/O.
func TestGoobsReadSurfaceCompiles(t *testing.T) {
	var _ func(*general.Client, ...*general.GetVersionParams) (*general.GetVersionResponse, error) = (*general.Client).GetVersion
	var _ func(*inputs.Client, ...*inputs.GetInputListParams) (*inputs.GetInputListResponse, error) = (*inputs.Client).GetInputList
	var _ func(*inputs.Client, ...*inputs.GetInputMuteParams) (*inputs.GetInputMuteResponse, error) = (*inputs.Client).GetInputMute
	var _ func(*inputs.Client, ...*inputs.GetInputAudioTracksParams) (*inputs.GetInputAudioTracksResponse, error) = (*inputs.Client).GetInputAudioTracks
	var _ func(*scenes.Client, ...*scenes.GetSceneListParams) (*scenes.GetSceneListResponse, error) = (*scenes.Client).GetSceneList
	var _ func(*scenes.Client, ...*scenes.GetCurrentProgramSceneParams) (*scenes.GetCurrentProgramSceneResponse, error) = (*scenes.Client).GetCurrentProgramScene
	var _ func(*sceneitems.Client, ...*sceneitems.GetSceneItemListParams) (*sceneitems.GetSceneItemListResponse, error) = (*sceneitems.Client).GetSceneItemList
	var _ func(*sceneitems.Client, *sceneitems.GetSceneItemEnabledParams) (*sceneitems.GetSceneItemEnabledResponse, error) = (*sceneitems.Client).GetSceneItemEnabled
	var _ func(*stream.Client, ...*stream.GetStreamStatusParams) (*stream.GetStreamStatusResponse, error) = (*stream.Client).GetStreamStatus

	var input typedefs.Input
	input.InputName = "Mic/Aux"
	input.InputUuid = "uuid"
	var tracks typedefs.InputAudioTracks = typedefs.InputAudioTracks{"track1": true}
	var item typedefs.SceneItem
	item.SceneItemID = 7
	item.SourceName = "Camera"
	item.SceneItemEnabled = true
	_ = input
	_ = tracks
	_ = item
}
