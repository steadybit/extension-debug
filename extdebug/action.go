/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package extdebug

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/extfile"
	"github.com/steadybit/extension-kit/extutil"
	"os"
	"sync"
)

type debugAction struct{}

// Make sure action implements all required interfaces
var (
	_ action_kit_sdk.Action[DebugActionState]           = (*debugAction)(nil)
	_ action_kit_sdk.ActionWithStatus[DebugActionState] = (*debugAction)(nil) // Optional, needed when the action needs a status method
	_ action_kit_sdk.ActionWithStop[DebugActionState]   = (*debugAction)(nil) // Optional, needed when the action needs a stop method

	debugRuns sync.Map
)

type DebugRun struct {
	Finished  bool
	ResultZip string
}

type DebugActionState struct {
	WorkingDir  string
	ExecutionId uuid.UUID
}

type DebugActionConfig struct {
}

func NewDebugAction() action_kit_sdk.Action[DebugActionState] {
	return &debugAction{}
}

func (l *debugAction) NewEmptyState() DebugActionState {
	return DebugActionState{}
}

// Describe returns the action description for the platform with all required information.
func (l *debugAction) Describe() action_kit_api.ActionDescription {
	return action_kit_api.ActionDescription{
		Id:          fmt.Sprint(actionID),
		Label:       "Debug Logs",
		Description: "Collects debug information",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(actionIcon),
		Category:    extutil.Ptr("Debug"),
		TargetSelection: extutil.Ptr(action_kit_api.TargetSelection{
			TargetType:          clusterTargetType,
			QuantityRestriction: extutil.Ptr(action_kit_api.All),
			SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "default",
					Description: extutil.Ptr("Find service by cluster"),
					Query:       "k8s.cluster-name=\"\"",
				},
			}),
		}),
		Kind: action_kit_api.Other,

		// How the action is controlled over time.
		//   External: The agent takes care and calls stop then the time has passed. Requires a duration parameter. Use this when the duration is known in advance.
		//   Internal: The action has to implement the status endpoint to signal when the action is done. Use this when the duration is not known in advance.
		//   Instantaneous: The action is done immediately. Use this for actions that happen immediately, e.g. a reboot.
		TimeControl: action_kit_api.TimeControlInternal,

		// The parameters for the action
		Parameters: []action_kit_api.ActionParameter{},
		Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: extutil.Ptr("1s"),
		}),
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

func (l *debugAction) Prepare(_ context.Context, state *DebugActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	log.Debug().Msg("Debug action **prepare**")
	var debugActionConfig DebugActionConfig
	if err := extconversion.Convert(request.Config, &debugActionConfig); err != nil {
		return nil, extension_kit.ToError("Failed to unmarshal the config.", err)
	}
	state.ExecutionId = request.ExecutionId

	temp, err := os.MkdirTemp("/tmp", "debugging_"+state.ExecutionId.String())
	if err != nil {
		log.Err(err).Msg("Failed to create temp dir")
		return nil, err
	}
	state.WorkingDir = temp
	debugRuns.Store(state.ExecutionId, DebugRun{
		Finished: false,
	})

	return nil, nil
}

func (l *debugAction) Start(_ context.Context, state *DebugActionState) (*action_kit_api.StartResult, error) {
	log.Info().Msg("Debug action **start**")

	go func() {
		resultZip := RunSteadybitDebug(state.WorkingDir)
		debugRuns.Store(state.ExecutionId, DebugRun{
			Finished:  true,
			ResultZip: resultZip,
		})
	}()

	return &action_kit_api.StartResult{}, nil
}

func (l *debugAction) Status(_ context.Context, state *DebugActionState) (*action_kit_api.StatusResult, error) {
	log.Debug().Msg("Debug action **status**")

	value, ok := debugRuns.Load(state.ExecutionId)
	if !ok {
		return nil, fmt.Errorf("state not found for execution id %s", state.ExecutionId)
	}
	if ok {
		debugRun := value.(DebugRun)
		if debugRun.Finished {
			log.Info().Msg("Debug action **finished**")
			artifacts := make([]action_kit_api.Artifact, 0)
			_, err := os.Stat(debugRun.ResultZip)

			if err == nil { // file exists
				content, err := extfile.File2Base64(debugRun.ResultZip)
				if err != nil {
					return nil, extutil.Ptr(extension_kit.ToError("Failed to open content file", err))
				}
				artifacts = append(artifacts, action_kit_api.Artifact{
					Label: "$(experimentKey)_$(executionId)_" + state.ExecutionId.String() + "_steadybit-debug.tar.gz",
					Data:  content,
				})
				log.Info().Msg("Uploading debug result: " + debugRun.ResultZip)
			}
			return &action_kit_api.StatusResult{
				Completed: true,
				Artifacts: extutil.Ptr(artifacts),
			}, nil
		}
	}
	return &action_kit_api.StatusResult{
		//indicate that the action is still running
		Completed: false,
	}, nil
}

func (l *debugAction) Stop(_ context.Context, state *DebugActionState) (*action_kit_api.StopResult, error) {
	log.Info().Msg("Debug action **stop**")

	err := os.RemoveAll(state.WorkingDir)
	if err != nil {
		log.Err(err).Msg("Failed to remove temp dir")
		return nil, err
	}
	return nil, nil
}
