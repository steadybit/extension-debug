// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"encoding/base64"
	"testing"

	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithMinikube(t *testing.T) {
	extFactory := e2e.HelmExtensionFactory{
		Name: "extension-debug",
		Port: 8089,
		ExtraArgs: func(m *e2e.Minikube) []string {
			return []string{
				"--set", "logging.level=debug",
			}
		},
	}

	e2e.WithDefaultMinikube(t, &extFactory, []e2e.WithMinikubeTestCase{
		{
			Name: "run debug",
			Test: testRunDebug,
		},
	})
}

// testRunDebug drives the debug action through its whole cycle. Gathering the cluster information,
// packing it into an archive and handing that archive back as an artifact from the status endpoint
// is what this extension exists for.
func testRunDebug(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	config := struct{}{}
	exec, err := e.RunAction("com.steadybit.extension_debug.debug", nil, config, nil)
	require.NoError(t, err)

	// TimeControlInternal: nothing but the extension itself ends this run.
	require.NoError(t, exec.Wait())

	artifacts := exec.Artifacts()
	require.Len(t, artifacts, 1, "the run must hand back exactly one debug archive")
	assert.Contains(t, artifacts[0].Label, "steadybit-debug.tar.gz")

	// The log line this replaces proved a path existed, not that the archive travelled intact.
	archive, err := base64.StdEncoding.DecodeString(artifacts[0].Data)
	require.NoError(t, err, "artifact data must be valid base64")
	require.Greater(t, len(archive), 2, "artifact data must not be empty")
	assert.Equal(t, []byte{0x1f, 0x8b}, archive[:2], "artifact data must be a gzip stream")
}
