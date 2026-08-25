/*
 * Copyright 2024 steadybit GmbH. All rights reserved.
 */

package extdebug

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
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

// awaitCompleted polls Status until the run reports itself completed. Start hands the gather to a
// goroutine, so there is nothing else to synchronise on.
func awaitCompleted(t *testing.T, action *debugAction, state *DebugActionState) *action_kit_api.StatusResult {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		result, err := action.Status(context.Background(), state)
		require.NoError(t, err)
		if result.Completed {
			return result
		}
		if time.Now().After(deadline) {
			t.Fatal("the run never reported itself completed")
		}
		time.Sleep(10 * time.Millisecond)
	}
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

	assert.DirExists(t, dir, "while a gather is in flight Stop must leave WorkingDir to the goroutine; removing it under an in-flight tar would crash the process")
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

	assert.DirExists(t, dir, "duplicate Stop must not remove WorkingDir mid-gather")
	run.mu.Lock()
	assert.False(t, run.cleaned, "duplicate Stop must not remove WorkingDir mid-gather")
	run.mu.Unlock()
}

func TestStatusReturnsTheArchiveAsArtifact(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "steadybit-debug.tar.gz")
	require.NoError(t, os.WriteFile(archive, []byte("archive contents"), 0600))

	action := &debugAction{gather: func(string) string { return archive }}
	_, state := registerRun(t, &debugRun{workingDir: dir})

	_, err := action.Start(context.Background(), state)
	require.NoError(t, err)

	result := awaitCompleted(t, action, state)
	require.NotNil(t, result.Artifacts)
	require.Len(t, *result.Artifacts, 1)
	artifact := (*result.Artifacts)[0]
	assert.Contains(t, artifact.Label, state.ExecutionId.String(), "the platform keys the artifact by execution")
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte("archive contents")), artifact.Data)
}

func TestStatusCompletesWithoutArtifactWhenGatherPanics(t *testing.T) {
	action := &debugAction{gather: func(string) string { panic("gather exploded") }}
	_, state := registerRun(t, &debugRun{workingDir: t.TempDir()})

	_, err := action.Start(context.Background(), state)
	require.NoError(t, err)

	// A panic in the gather must neither take the extension process down nor leave the run polling
	// forever. Note what the run reports today: completed, no artifact, and no error or message -
	// so the platform shows a green step and the user is left without a bundle and without an
	// explanation. That is current behaviour, not desired behaviour.
	result := awaitCompleted(t, action, state)
	assert.Empty(t, result.Artifacts)
}

func TestGatherRemovesWorkingDirWhenStoppedInFlight(t *testing.T) {
	dir := t.TempDir()
	entered := make(chan struct{})
	release := make(chan struct{})
	action := &debugAction{gather: func(string) string {
		close(entered)
		<-release
		return filepath.Join(dir, "steadybit-debug.tar.gz")
	}}
	_, state := registerRun(t, &debugRun{workingDir: dir})

	_, err := action.Start(context.Background(), state)
	require.NoError(t, err)
	<-entered

	_, err = action.Stop(context.Background(), state)
	require.NoError(t, err)
	require.DirExists(t, dir, "Stop must leave WorkingDir alone while the tar is still running")

	close(release)
	// The gather has returned, so the cleanup Stop had to skip is now the goroutine's job.
	assert.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.NoDirExists(c, dir)
	}, 5*time.Second, 10*time.Millisecond, "the gather goroutine must remove WorkingDir once it observes the stop")
}
