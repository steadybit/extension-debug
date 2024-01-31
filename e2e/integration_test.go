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
			Test: testRundebug,
		},
	})
}

func testRundebug(t *testing.T, m *e2e.Minikube, e *e2e.Extension) {
	config := struct{}{}
	exec, err := e.RunAction("com.steadybit.extension_debug.debug", nil, config, nil)
	require.NoError(t, err)
	e2e.AssertLogContains(t, m, e.Pod, "Debug action **start**")
	require.NoError(t, exec.Cancel())
}
