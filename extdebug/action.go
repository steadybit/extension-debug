/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package extdebug

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/action-kit/go/action_kit_sdk"
	extension_kit "github.com/steadybit/extension-kit"
	"github.com/steadybit/extension-kit/extbuild"
	"github.com/steadybit/extension-kit/extconversion"
	"github.com/steadybit/extension-kit/extutil"
)

type debugAction struct{}

// Make sure action implements all required interfaces
var (
	_ action_kit_sdk.Action[DebugActionState]           = (*debugAction)(nil)
	_ action_kit_sdk.ActionWithStatus[DebugActionState] = (*debugAction)(nil) // Optional, needed when the action needs a status method
	_ action_kit_sdk.ActionWithStop[DebugActionState]   = (*debugAction)(nil) // Optional, needed when the action needs a stop method
)

type DebugActionState struct {
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
		Id:          fmt.Sprintf(actionID),
		Label:       "debug",
		Description: "collects debug information",
		Version:     extbuild.GetSemverVersionStringOrUnknown(),
		Icon:        extutil.Ptr(actionIcon),
		// Category for the targets to appear in
		Category: extutil.Ptr("other"),

		// To clarify the purpose of the action, you can set a kind.
		//   Attack: Will cause harm to targets
		//   Check: Will perform checks on the targets
		//   LoadTest: Will perform load tests on the targets
		//   Other
		Kind: action_kit_api.Other,

		// How the action is controlled over time.
		//   External: The agent takes care and calls stop then the time has passed. Requires a duration parameter. Use this when the duration is known in advance.
		//   Internal: The action has to implement the status endpoint to signal when the action is done. Use this when the duration is not known in advance.
		//   Instantaneous: The action is done immediately. Use this for actions that happen immediately, e.g. a reboot.
		TimeControl: action_kit_api.TimeControlInternal,

		// The parameters for the action
		Parameters: []action_kit_api.ActionParameter{},
		//Status: extutil.Ptr(action_kit_api.MutatingEndpointReferenceWithCallInterval{
		//	CallInterval: extutil.Ptr("1s"),
		//}),
		Stop: extutil.Ptr(action_kit_api.MutatingEndpointReference{}),
	}
}

// Prepare is called before the action is started.
// It can be used to validate the parameters and prepare the action.
// It must not cause any harmful effects.
// The passed in state is included in the subsequent calls to start/status/stop.
// So the state should contain all information needed to execute the action and even more important: to be able to stop it.
func (l *debugAction) Prepare(_ context.Context, state *DebugActionState, request action_kit_api.PrepareActionRequestBody) (*action_kit_api.PrepareResult, error) {
	log.Info().Msg("Debug action **prepare**")
	var config DebugActionConfig
	if err := extconversion.Convert(request.Config, &config); err != nil {
		return nil, extension_kit.ToError("Failed to unmarshal the config.", err)
	}

	return &action_kit_api.PrepareResult{}, nil
}

// Start is called to start the action
// You can mutate the state here.
// You can use the result to return messages/errors/metrics or artifacts
func (l *debugAction) Start(_ context.Context, state *DebugActionState) (*action_kit_api.StartResult, error) {
	log.Info().Msg("Debug action **start**")

	return &action_kit_api.StartResult{}, nil
}

// Status is optional.
// If you implement that it will be called periodically to check the status of the action.
// You can use the result to signal that the action is done and to return messages/errors/metrics or artifacts
func (l *debugAction) Status(_ context.Context, state *DebugActionState) (*action_kit_api.StatusResult, error) {
	log.Info().Msg("Debug action **status**")

	return &action_kit_api.StatusResult{
		//indicate that the action is still running
		Completed: false,
		//These messages will show up in agent log
	}, nil
}

// Stop is called to stop the action
// It will be called even if the start method did not complete successfully.
// It should be implemented in a immutable way, as the agent might to retries if the stop method timeouts.
// You can use the result to return messages/errors/metrics or artifacts
func (l *debugAction) Stop(_ context.Context, state *DebugActionState) (*action_kit_api.StopResult, error) {
	log.Info().Msg("Debug action **stop**")

	return &action_kit_api.StopResult{}, nil
}
