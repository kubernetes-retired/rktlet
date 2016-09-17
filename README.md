# rktlet - The rkt implementation of a Kubernetes Container Runtime

The rktlet repository contains design and code related to letting the Kubelet run containers with the rkt container runtime.

The work in this repository is meant to eventually supplant the [rkt package](https://github.com/kubernetes/kubernetes/tree/v1.3.0/pkg/kubelet/rkt/) in the main Kubernetes repository.

## Current Status

Currently the project is under design and development of the next iteration of integration between rkt and the Kubelet.
However, the current functional integration of rkt into Kubernetes lives in the above kubelet package, not here.

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- Kubernetes Community: https://github.com/kubernetes/community/tree/master/sig-rktnetes
- Slack: #sig-rktnetes
- Mailing List: https://groups.google.com/forum/#!forum/kubernetes-sig-rktnetes

## Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The project was established 2016-01-02. The incubator team for the project is:

- Sponsor: Tim Hockin (@thockin)
- Champion: Yu-Ju Hong (@yujuhong)
- SIG: sig-rktnetes &amp; sig-node

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
