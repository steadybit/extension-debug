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

	// debugRuns maps an execution id to its *debugRun.
	debugRuns sync.Map
)

// debugRun is the single source of truth for one debug execution, guarded by mu. Keeping
// the lifecycle flags together under one lock lets Start, Status, Stop and the gather
// goroutine make the "is the gather still tarring WorkingDir?" decision atomically — which
// determines who may remove WorkingDir without racing steadybit-debug's tar (a failed tar
// makes it os.Exit(1) and crashes the whole extension).
type debugRun struct {
	mu         sync.Mutex
	workingDir string
	started    bool // the gather goroutine has been launched
	gatherDone bool // RunSteadybitDebug has returned, so no tar is running anymore
	stopped    bool // Stop has been called
	cleaned    bool // workingDir has been removed (removal is idempotent)
	finished   bool // the result is ready for Status
	resultZip  string
}

// removeWorkingDir deletes the working directory at most once. The caller must hold mu.
func (r *debugRun) removeWorkingDir() {
	if r.cleaned {
		return
	}
	r.cleaned = true
	if err := os.RemoveAll(r.workingDir); err != nil {
		log.Err(err).Msg("Failed to remove temp dir")
	}
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
		Icon:        new(actionIcon),
		Technology:  new("Debug"),
		TargetSelection: new(action_kit_api.TargetSelection{
			TargetType:          clusterTargetType,
			QuantityRestriction: extutil.Ptr(action_kit_api.QuantityRestrictionAll),
			SelectionTemplates: new([]action_kit_api.TargetSelectionTemplate{
				{
					Label:       "cluster name",
					Description: new("Find service by cluster"),
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
		Status: new(action_kit_api.MutatingEndpointReferenceWithCallInterval{
			CallInterval: new("1s"),
		}),
		Stop: new(action_kit_api.MutatingEndpointReference{}),
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
	debugRuns.Store(state.ExecutionId, &debugRun{workingDir: temp})

	return nil, nil
}

func (l *debugAction) Start(_ context.Context, state *DebugActionState) (*action_kit_api.StartResult, error) {
	log.Info().Msg("Debug action **start**")

	value, ok := debugRuns.Load(state.ExecutionId)
	if !ok {
		return nil, fmt.Errorf("state not found for execution id %s", state.ExecutionId)
	}
	run := value.(*debugRun)

	run.mu.Lock()
	run.started = true
	run.mu.Unlock()

	go func() {
		// Recover so a panic while gathering debug information cannot crash the whole
		// extension process (and take down other in-flight actions). Mark the run finished
		// so Status completes instead of polling forever.
		defer func() {
			if r := recover(); r != nil {
				log.Error().Msgf("Recovered from panic while gathering debug information: %v", r)
				run.mu.Lock()
				run.gatherDone = true
				run.finished = true
				if run.stopped {
					run.removeWorkingDir()
				}
				run.mu.Unlock()
			}
		}()
		resultZip := RunSteadybitDebug(state.WorkingDir)

		run.mu.Lock()
		run.gatherDone = true
		if run.stopped {
			// Stopped while gathering: the result archive (which lives inside WorkingDir)
			// is no longer wanted, and Stop left WorkingDir for this goroutine to remove —
			// Stop couldn't, because removing WorkingDir while the tar above was still
			// running would make steadybit-debug os.Exit(1) and crash the whole extension.
			// The tar has returned, so removing it here is now safe.
			run.removeWorkingDir()
		} else {
			run.finished = true
			run.resultZip = resultZip
		}
		run.mu.Unlock()
	}()

	return &action_kit_api.StartResult{}, nil
}

func (l *debugAction) Status(_ context.Context, state *DebugActionState) (*action_kit_api.StatusResult, error) {
	log.Debug().Msg("Debug action **status**")

	value, ok := debugRuns.Load(state.ExecutionId)
	if !ok {
		return nil, fmt.Errorf("state not found for execution id %s", state.ExecutionId)
	}
	run := value.(*debugRun)
	run.mu.Lock()
	finished := run.finished
	resultZip := run.resultZip
	run.mu.Unlock()

	if finished {
		log.Info().Msg("Debug action **finished**")
		artifacts := make([]action_kit_api.Artifact, 0)
		_, err := os.Stat(resultZip)

		if err == nil { // file exists
			content, err := extfile.File2Base64(resultZip)
			if err != nil {
				return nil, new(extension_kit.ToError("Failed to open content file", err))
			}
			artifacts = append(artifacts, action_kit_api.Artifact{
				Label: "$(experimentKey)_$(executionId)_" + state.ExecutionId.String() + "_steadybit-debug.tar.gz",
				Data:  content,
			})
			log.Info().Msg("Uploading debug result: " + resultZip)
		}
		return &action_kit_api.StatusResult{
			Completed: true,
			Artifacts: new(artifacts),
		}, nil
	}
	return &action_kit_api.StatusResult{
		//indicate that the action is still running
		Completed: false,
	}, nil
}

func (l *debugAction) Stop(_ context.Context, state *DebugActionState) (*action_kit_api.StopResult, error) {
	log.Info().Msg("Debug action **stop**")

	value, ok := debugRuns.LoadAndDelete(state.ExecutionId)
	if !ok {
		// Already stopped, or never prepared. Returning here also makes a duplicate Stop
		// (e.g. a platform retry) a no-op, so it cannot remove WorkingDir while a still
		// in-flight gather goroutine is tarring it.
		return nil, nil
	}
	run := value.(*debugRun)

	run.mu.Lock()
	defer run.mu.Unlock()
	run.stopped = true
	// Remove WorkingDir (which holds the result archive) only when no gather goroutine
	// could still be tarring it: either none was started, or it has already finished.
	// While a gather is in flight, leave WorkingDir to the goroutine, which removes it
	// once RunSteadybitDebug returns and it observes stopped — removing it here under an
	// in-flight tar would make steadybit-debug os.Exit(1) and crash the whole extension.
	if !run.started || run.gatherDone {
		run.removeWorkingDir()
	}
	return nil, nil
}
