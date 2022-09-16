# Kubernetes event logger

<img src="https://raw.githubusercontent.com/max-rocket-internet/k8s-event-logger/master/img/k8s-logo.png" width="100">

This tool simply watches Kubernetes Events and logs them to stdout in JSON to be collected and stored by your logging solution, e.g. [fluentd](https://github.com/fluent/fluentd-kubernetes-daemonset) or [fluent-bit](https://fluentbit.io/). Other tools exist for persisting Kubernetes Events, such as Sysdig, Datadog or Google's [event-exporter](https://github.com/GoogleCloudPlatform/k8s-stackdriver/tree/master/event-exporter) but this tool is open and will work with any logging solution.

### Why?

Events in Kubernetes log very important information. If are trying to understand what happened in the past then these events show clearly what your Kubernetes cluster was thinking and doing. Some examples:

- Pod events like failed probes, crashes, scheduling related information like `TriggeredScaleUp` or `FailedScheduling`
- HorizontalPodAutoscaler events like scaling up and down
- Deployment events like scaling in and out of ReplicaSets
- Ingress events like create and update

The problem is that these events are simply API objects in Kubernetes and are only stored for about 1 hour. Without some way of storing these events, debugging a problem in the past very tricky.

Example of events:

```
39m   Normal  UpdatedLoadBalancer      Service     Updated load balancer with new hosts
40m   Normal  SuccessfulDelete         DaemonSet   Deleted pod: ingress02-nginx-ingress-controller-vqqjp
41m   Normal  ScaleDown                Node        node removed by cluster autoscaler
54m   Normal  Started                  Pod         Started container
55m   Normal  Starting                 Node        Starting kubelet.
55m   Normal  Starting                 Node        Starting kube-proxy.
55m   Normal  NodeAllocatableEnforced  Node        Updated Node Allocatable limit across pods
55m   Normal  NodeReady                Node        Node ip-10-0-23-14.compute.internal status is now: NodeReady
58m   Normal  SuccessfulCreate         DaemonSet   Created pod: ingress02-nginx-ingress-controller-bz7xj
58m   Normal  CREATE                   ConfigMap   ConfigMap default/ingress02-nginx-ingress-controller
```

### Installation

Use the [Helm](https://helm.sh/) chart:

```
helm install chart/
```

Or use the docker image [maxrocketinternet/k8s-event-logger][image]

[image]: https://hub.docker.com/r/maxrocketinternet/k8s-event-logger

#### Building a container image

If you're unable to use the [prebuilt][image] docker image, you can build it yourself:

```sh
make IMG=your.docker.registry/k8s-event-logger image
```

This uses `docker buildx` to create a [multi-platform image][]. To set up your build host system to be able to build these images, see [this guide][qemu-binfmt].

[multi-platform image]: https://docs.docker.com/build/building/multi-platform/
[qemu-binfmt]: https://docs.nvidia.com/datacenter/cloud-native/playground/x-arch.html

### Testing

Run it:

```
go run main.go
```
