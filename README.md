# Steadybit extension-debug

Run [steadybit-debug](https://github.com/steadybit/steadybit-debug) as extension for gather denug information of agent and extension.

## Configuration

| Environment Variable                                      | Helm value                           | Meaning                                                                                                               | Required | Default                 |
|-----------------------------------------------------------|--------------------------------------|-----------------------------------------------------------------------------------------------------------------------|----------|-------------------------|

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
