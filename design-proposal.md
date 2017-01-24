# Proposal: Design of the rkt + Kubernetes CRI

## Abstract

This proposal can be viewed as more a discussion than an actual proposal, because we still haven't reached a consensus on the architecture.

## Backgroud

As container runtime interface (CRI) proposals and original stub codes are already merged into Kubernetes, the actual work has already begun.
All existing and future container runtimes are expected to conform the interface and provides feature parities.

## Design(s)

There are three different designs:

- `Kubelet` --_gPRC calls_--> `rktlet service` --_invoke 'rkt' binary, or use some rkt library_--> result

- `Kubelet` --_in process invoke_--> `rktshim` --_gPRC calls_--> `rkt API service` --> result

- `Kubelet` --_gPRC calls_--> `rktlet service` --_gPRC calls_--> `rkt API service` --> result

1. The major difference of the first two approaches is the 1st approach doesn't require any rkt code to live in the kubernetes codebase, which improves the Kubelet's modularity as a whole.
However, it appears the rktlet code will live outside of rkt project as well since the gRPC interfaces are Kubernetes specific.
This means for the 1st approach, we protentially have more work to do, and many of them are duplicated effort with the current rkt api service.
Also I don't think fork/exec a rkt process is the best ideal state for the rktnetes integration, so I would expect we export a lot of rkt functionalities as a library.

2. For the second approach, it will require some shim code compiled with Kubelet so as to convert Kubernetes pod/container operations into rkt pod/app operations.
Other than that, we can have the actual CRI implementation in the rkt API service, then we get the setup code and the rkt internal data structrue, functions for free.

3. The third approach is somehow a combination of the first two.
It has the benefits of the both worlds, rktlet lives outside of Kubelet, so it could lead to higher iteration speed, smaller Kubelet size, and better modularity.
Also on the other side, the 'rkt API service' is compiled with rkt, and communicate with the rktlet through gRPC, so we still can use a lot of the existing rkt facilities.
The potential con is that instead of one gPRC service, now we introduce two, which may or may not cause some overhead.
Even despite the overhead of one more gRPC hop, we still need to consider the overhead of packaging this one more gRPC daemon, and the versioning scheme.

## What should be in rktlet repository

This highly depends what design path above we choose to go through.

1. For the first approach, rktlet repo will have the CRI server gRPC setup code, handlers, the actual CRI implementation, and potentially some rkt library (or include them as third party).

2. For the second approach, rktlet will ultimately have nothing substantial, the actual code lives in `k8s.io/kubelet/rkt package`, and rkt api services.

3. For the third approach, rktlet repo will have the CRI server gRPC setup code, handlers, but the actual CRI implementation will live in rkt API service, and rktlet service will call into it via the rkt gRPC API.

In all these cases, rktlet will contain necessary documentations, like the design proposals like this one.


## Current state

Current, there are several PRs against the `k8s/kubelet/rkt`, `k8s/kubelet/rktshim` package on the fly to add rktshim codes, for the image services which is already able to be implemented today without more rkt upstream support.

https://github.com/kubernetes/kubernetes/pull/29914 (Kubelet rkt CRI ImageService)
https://github.com/kubernetes/kubernetes/pull/29919 (Kubelet rkt CRI stubs & fakes)
https://github.com/kubernetes/kubernetes/pull/30513 (Kubelet rkt CRI uses ImageService)

The image service implmentation proposed in [#kubernetes/30513](https://github.com/kubernetes/kubernetes/pull/30513) is forking/execing the rkt binary.
There is also an ongoing work on implementing the image services on the rkt API service side:

https://github.com/coreos/rkt/pull/3073 (rkt: Implement FetchImages and RemoveImages API through API service)

For app-level operations, the rkt upstream already merged the proposal [#rkt/2932](https://github.com/coreos/rkt/pull/2932).
The proposal defines some new stage1 entrypoints to provide app-level operations, it also provides examples of new rkt CLI commands for executing these entrypoints.
Obviously rkt API service is able to provide such interfaces as well.

Besides the proposal, there is no actual implementation PR yet.
