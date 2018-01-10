# Supported Kubernetes features

This is an uncomplete list of Kubernetes features supported by rktlet.

| Feature                | Supported | Comments |
| ---------------------- |:---------:| -------- |
| Pod lifecycle          | YES       |          |
| Container lifecycle    | YES       |          |
| Logging                | YES       |          |
| `kubectl attach`       | NO        | See [#8](https://github.com/kubernetes-incubator/rktlet/issues/8) |
| `kubectl port-forward` | NO        |          |
| `kubectl exec`         | YES       |          |
| SELinux                | NO        |          |
| Seccomp                | Partial   | Custom local profiles don't work ([#159](https://github.com/kubernetes-incubator/rktlet/issues/159)). |
| Host networking        | YES       |          |
| CNI networking         | YES       | rkt only supports CNI v0.3.0 ([#3600](https://github.com/rkt/rkt/issues/3600)). |
| Empty volumes          | YES       |          |
| Host volumes           | YES       |          |
