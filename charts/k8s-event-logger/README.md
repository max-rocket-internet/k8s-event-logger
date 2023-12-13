# k8s-event-logger

This chart installs [github.com/max-rocket-internet/k8s-event-logger](https://github.com/max-rocket-internet/k8s-event-logger).

## Prerequisites

- Kubernetes 1.23+

## Installing the Chart

To install the chart with the release name `my-release` and default configuration:

```sh
helm install my-release ./chart
```

## Uninstalling the Chart

To delete the chart:

```sh
helm delete my-release
```

## Configuration

The following table lists the configurable parameters for this chart and their default values.

| Parameter                | Description                          | Default                                                |
| -------------------------|--------------------------------------|--------------------------------------------------------|
| `resources`              | Resources for the overprovision pods | `{}`                                                   |
| `image.repository`       | Image repository                     | `maxrocketinternet/k8s-event-logger`                   |
| `image.tag`              | Image tag                            | `2.0`                                                  |
| `image.pullPolicy`       | Container pull policy                | `IfNotPresent`                                         |
| `affinity`               | Map of node/pod affinities           | `{}`                                                   |
| `nodeSelector`           | Node labels for pod assignment       | `{}`                                                   |
| `annotations`            | Optional deployment annotations      | `{}`                                                   |
| `fullnameOverride`       | Override the fullname of the chart   | `nil`                                                  |
| `nameOverride`           | Override the name of the chart       | `nil`                                                  |
| `tolerations`            | Optional deployment tolerations      | `[]`                                                   |
| `podLabels`              | Additional labels to use for pods    | `{}`                                                   |
| `env.KUBERNETES_API_URL` | URL of the k8s API in your cluster   | `https://172.20.0.1:443`                               |
| `env.CA_FILE`            | Path to the service account CA file  | `/var/run/secrets/kubernetes.io/serviceaccount/ca.crt` |
| `podLabels`              | Additional labels to use for pods    | `{}`                                                   |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install` or provide a YAML file containing the values for the above parameters:

```sh
helm install --name my-release stable/k8s-event-logger --values values.yaml
```
