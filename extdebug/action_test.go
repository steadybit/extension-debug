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

func registerRun(t *testing.T, run *debugRun) (uuid.UUID, *DebugActionState) {
	t.Helper()
	id := uuid.New()
	debugRuns.Store(id, run)
	t.Cleanup(func() { debugRuns.Delete(id) })
	return id, &DebugActionState{ExecutionId: id, WorkingDir: run.workingDir}
}

func assertRemoved(t *testing.T, dir string) {
	t.Helper()
	_, err := os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "WorkingDir should have been removed")
}

func TestStopRemovesWorkingDirWhenGatherNeverStarted(t *testing.T) {
	dir := t.TempDir()
	_, state := registerRun(t, &debugRun{workingDir: dir})

	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	assertRemoved(t, dir)
}

func TestStopRemovesWorkingDirWhenGatherFinished(t *testing.T) {
	dir := t.TempDir()
	_, state := registerRun(t, &debugRun{workingDir: dir, started: true, gatherDone: true, finished: true})

	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	assertRemoved(t, dir)
}

func TestStopLeavesWorkingDirWhileGatherInFlight(t *testing.T) {
	dir := t.TempDir()
	run := &debugRun{workingDir: dir, started: true}
	_, state := registerRun(t, run)

	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr, "while a gather is in flight Stop must leave WorkingDir to the goroutine; removing it under an in-flight tar would crash the process")
	run.mu.Lock()
	assert.True(t, run.stopped, "the gather goroutine must be signalled to discard its result")
	assert.False(t, run.cleaned, "Stop must not remove WorkingDir while the gather is in flight")
	run.mu.Unlock()
}

func TestDuplicateStopDoesNotRemoveWorkingDirMidGather(t *testing.T) {
	dir := t.TempDir()
	run := &debugRun{workingDir: dir, started: true}
	_, state := registerRun(t, run)

	_, err := (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)
	// A retried Stop (the first already consumed the run) must be a no-op — it must not
	// remove WorkingDir while the first call's gather goroutine may still be tarring it.
	_, err = (&debugAction{}).Stop(context.Background(), state)
	require.NoError(t, err)

	_, statErr := os.Stat(dir)
	assert.NoError(t, statErr, "duplicate Stop must not remove WorkingDir mid-gather")
	run.mu.Lock()
	assert.False(t, run.cleaned, "duplicate Stop must not remove WorkingDir mid-gather")
	run.mu.Unlock()
}
