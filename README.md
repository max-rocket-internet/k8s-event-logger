# Kubernetes event logger

Events in Kubernetes log very important information. If are trying to understand what happened in the past then these events show clearly what your Kubernetes cluster was thinking and doing. Some examples:

- Pod events like failed probes, crashes, scheduling related information like `TriggeredScaleUp` or `FailedScheduling`
- HorizontalPodAutoscaler events like scaling up and down
- Deployment events like scaling in and out of ReplicaSets
- Ingress events like create and update

The problem is that these events are simply API objects in Kubernetes and are only stored for about 1 hour. This can make debugging a problem in the past very tricky.

This simple container and [Helm](https://helm.sh/) chart will run in your cluster, watch for events and print them to stdout in JSON. The assumption is that you already have a daemonset for collecting all pod logs and sending them to a central system, e.g. ELK, Splunk, Graylog etc.

It's based on work in these 2 repositories:

- https://github.com/splunk/fluent-plugin-kubernetes-objects
- https://github.com/fluent/fluentd-kubernetes-daemonset

### Installation

Use the [Helm](https://helm.sh/) chart:

```
helm install chart/
```
