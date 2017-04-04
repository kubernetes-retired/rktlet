# rktlet - rkt-based implementation of Kubernetes Container Runtime Interface

rktlet is a Kubernetes [Container Runtime Interface][k8s-cri] implementation using [rkt][rkt] as the main container runtime.
rkt is an ongoing [CNCF effort][rkt-cncf] to develop a pod-native container runtime.

The goal of this project is to eventually supplant the [rkt package](https://github.com/kubernetes/kubernetes/tree/v1.3.0/pkg/kubelet/rkt/) in the main Kubernetes repository.

This repository contains all design documents and code for the project.

Please note that the current (non-CRI) integration of rkt into Kubernetes lives in the kubelet package, not here.

[rkt][https://github.com/coreos/rkt]
[k8s-cri][http://blog.kubernetes.io/2016/12/container-runtime-interface-cri-in-kubernetes.html]
[rkt-cncf][https://www.cncf.io/announcement/2017/03/29/cloud-native-computing-foundation-becomes-home-pod-native-container-engine-project-rkt/]

## Current Status

Currently the project is under development and being tested against Kubernetes HEAD.

Kubernetes tracking:

- [Conformance testing](https://github.com/kubernetes-incubator/rktlet/issues/95): 89/94 passing as of 2017-02-08

## Community, discussion, contribution, and support

Learn how to engage with the Kubernetes community on the [community page](http://kubernetes.io/community/).

You can reach the maintainers of this project at:

- Kubernetes Community: https://github.com/kubernetes/community/tree/master/sig-rktnetes
- Slack: #sig-node-rkt
- Mailing List: https://groups.google.com/forum/#!forum/kubernetes-sig-rktnetes

## Kubernetes Incubator

This is a [Kubernetes Incubator project](https://github.com/kubernetes/community/blob/master/incubator.md). The project was established 2016-01-02. The incubator team for the project is:

- Sponsor: Tim Hockin (@thockin)
- Champion: Yu-Ju Hong (@yujuhong)
- SIG: sig-rktnetes &amp; sig-node

### Code of conduct

Participation in the Kubernetes community is governed by the [Kubernetes Code of Conduct](code-of-conduct.md).
