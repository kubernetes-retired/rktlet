# journal2cri

This is a small daemon that converts a given pod's journal to the format described in [this proposal](https://github.com/kubernetes/kubernetes/pull/34376).

It operates as an additional binary which will be injected into each pod as an additional applicaiton. This application will be started along with the sandbox and will not be visible to the kubelet.

## Intended status

This is not the intended long term solution. This is a hack. The long term solution will include additional development rkt-side related to the attach work there.

This is being done as a means to have a relatively sane answer fairly quickly.
