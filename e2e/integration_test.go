// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2023 Steadybit GmbH

package e2e

import (
	"github.com/steadybit/action-kit/go/action_kit_test/e2e"
	"github.com/stretchr/testify/require"
	"testing"
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

	// The action is TimeControlInternal, so nothing but the extension itself ends this run.
	require.NoError(t, exec.Wait())

	// Wait returning already proves the run completed; this log line is the part that cannot be
	// observed through the action API, because action-kit's test client drops the status result's
	// artifacts. It is only written when an archive exists and got attached to that result.
	e2e.AssertLogContains(t, m, e.Pod, "Uploading debug result:")
}
