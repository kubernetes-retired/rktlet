# Proposal: Design of the rkt + Kubernetes CRI

## Background

Currently, the Kubernetes project supports rkt as a container runtime via an implementation in the kubelet [here](https://github.com/kubernetes/kubernetes/tree/v1.3.6/pkg/kubelet/rkt).
This implementation, for historical reasons, has required implementing a large amount of logic shared by the original Docker implementation.

In order to make additional container runtime integrations easier, more clearly defined, and more consistent, a new [Container Runtime Interface](https://github.com/kubernetes/features/issues/54) (CRI) is being designed.
The existing runtimes, in order to both prove the correctness of the interface and reduce maintenance burden, are incentivized to move to this interface.

This document proposes how the rkt runtime integration will transition to using the CRI.

## Goals

### Full-featured

The CRI integration must work as well as the existing integration in terms of features. Until that's the case, the existing integration will continue to be maintained.

### Easy to Deploy

The new integration should not be any more difficult to deploy and configure than the existing integration.

### Easy to Develop 

This iteration should be as easy to work and iterate on as the original one.

It will be available in an initial usable form quickly in order to validate the CRI.

## Design

In order to fulfill the above goals, the rkt CRI integration will make the following choices:

*TODO: Pretty picture goes here*

### Remain in-process with Kubelet

The current rkt container runtime integration is able to be deployed simply by deploying the kubelet binary. Similarly, the Docker integration (as visible in the [dockershim](https://github.com/kubernetes/kubernetes/tree/83035a52ce59d39f216079ebb3968e3d3b69085f/pkg/kubelet/dockershim) package) is making the choice to remain there.

This is, in no small part, to make it *Easy to Deploy*. Remaining in-process also helps this integration not regress on performance, one axis of being *Full-Featured*.

### Developed as a Separate Repository

Brian Grant's discussion on splitting the Kubernetes project into [separate repos](https://github.com/kubernetes/kubernetes/issues/24343) is a compelling argument for why it makes sense to split this work into a separate repo. In order to be *Easy to Develop*, this iteration will be maintained as a separate repository, and re-vendored back in.

This choice will also allow better long-term growth in terms of better
issue-management, testing pipelines, and so on. Unfortunately, in the short
term, it's possible that some aspects of this will also cause pain and it's
very difficult to weight each side correctly.

### Exec the rkt binary (initially)

While significant work on the rkt
[api-service](https://coreos.com/rkt/docs/latest/subcommands/api-service.html)
has been made, it has also been a source of problems and additional complexity,
and was never transitioned to entirely.

In addition, the rkt cli has historically been the primary interface to the rkt runtime. The initial integration will execute the rkt binary directly, other than for the `run` operation which will be parented by `systemd` (which will be assumed to exist in the initial implementation as well).

In the future, these decisions are expected to be changed such that rkt is vendored as a library dependency for all operations, and run is refactored to work under other init systems.

