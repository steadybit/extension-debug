/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package extdebug

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopRemovesWorkingDirWhenGatherNeverStarted(t *testing.T) {
	dir := t.TempDir()
	state := &DebugActionState{ExecutionId: uuid.New(), WorkingDir: dir}

	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "with no gather goroutine, Stop should remove WorkingDir")
}

func TestStopRemovesWorkingDirWhenGatherFinished(t *testing.T) {
	dir := t.TempDir()
	execId := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	debugCancels.Store(execId, cancel)
	debugRuns.Store(execId, DebugRun{Finished: true})

	state := &DebugActionState{ExecutionId: execId, WorkingDir: dir}
	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	assert.Error(t, ctx.Err(), "the gather goroutine should be signalled")
	_, statErr := os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "after the gather finished, Stop should remove WorkingDir (and the archive inside it)")
	_, present := debugCancels.Load(execId)
	assert.False(t, present, "Stop should drop the cancel entry")
}

func TestStopLeavesWorkingDirWhileGatherRunning(t *testing.T) {
	dir := t.TempDir()
	execId := uuid.New()
	ctx, cancel := context.WithCancel(context.Background())
	debugCancels.Store(execId, cancel)
	debugRuns.Store(execId, DebugRun{Finished: false})

	state := &DebugActionState{ExecutionId: execId, WorkingDir: dir}
	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	assert.Error(t, ctx.Err(), "the gather goroutine must be signalled to discard its result")
	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr, "while a gather is in flight, Stop must leave WorkingDir — removing it under an in-flight tar would crash the process; the goroutine removes it on completion")
	_, present := debugCancels.Load(execId)
	assert.False(t, present, "Stop should drop the cancel entry")
}
