# Steadybit extension-debug

TODO describe what your extension is doing here from a user perspective.

TODO optionally add your extension to the [Reliability Hub](https://hub.steadybit.com/) by creating
a [pull request](https://github.com/steadybit/reliability-hub-db) and add a link to this README.

## Configuration

| Environment Variable                                      | Helm value                           | Meaning                                                                                                               | Required | Default                 |
|-----------------------------------------------------------|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------|----------|-------------------------|
| `STEADYBIT_EXTENSION_ROBOT_NAMES`                         |                                      | Comma-separated list of discoverable robots                                                                           | yes      | Bender,Terminator,R2-D2 |
| `STEADYBIT_EXTENSION_DISCOVERY_ATTRIBUTES_EXCLUDES_ROBOT` | `discovery.attributes.excludes.robot | List of Robot Attributes which will be excluded during discovery. Checked by key equality and supporting trailing "*" | no       |                         |

The extension supports all environment variables provided by [steadybit/extension-kit](https://github.com/steadybit/extension-kit#environment-variables).

## Installation

### Using Docker

```sh
docker run \
  --rm \
  -p 8089 \
  --name steadybit-extension-debug \
  ghcr.io/steadybit/extension-debug:latest
```

### Using Helm in Kubernetes

```sh
helm repo add steadybit-extension-debug https://steadybit.github.io/extension-debug
helm repo update
helm upgrade steadybit-extension-debug \
    --install \
    --wait \
    --timeout 5m0s \
    --create-namespace \
    --namespace steadybit-extension \
    steadybit-extension-debug/steadybit-extension-debug
```

## Register the extension

Make sure to register the extension at the steadybit platform. Please refer to
the [documentation](https://docs.steadybit.com/integrate-with-steadybit/extensions/extension-installation) for more information.
