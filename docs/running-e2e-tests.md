## Running Kubernetes e2e tests

This document explains how to run Kubernetes e2e tests with rktlet.

First, start the rktlet, optionally with a more verbose output:

```
$ sudo ./bin/rktlet -v=4
```

On another terminal, go to the Kubernetes source directory and start a local cluster.
We set some flags because tests use those features (`ALLOW_*`) and we configure the cluster to use the CRI, and rktlet in particular.

```
$ cd $GOPATH/src/k8s.io/kubernetes
$ sudo ALLOW_PRIVILEGED=true \
       ALLOW_SECURITY_CONTEXT=true \
       CONTAINER_RUNTIME=remote \
       CONTAINER_RUNTIME_ENDPOINT=/var/run/rktlet.sock \
       CGROUP_DRIVER=systemd \
       ./hack/local-up-cluster.sh
```

On yet another terminal, go again to the Kubernetes source directory and start the tests.
In this example we run the conformance tests and enable running tests in parallel in order to save some time, skipping those that need serial execution _(note: usually more tests pass if we don't enable parallelism)_.
Finally, we redirect the test output to a file so you can see what's wrong in failing tests.

```
$ export KUBECONFIG=/var/run/kubernetes/admin.kubeconfig
$ export GINKGO_PARALLEL=y
$ go run hack/e2e.go -- --provider=local -v --test --test_args="--ginkgo.focus=\[Conformance\] --ginkgo.skip=\[Serial\]" > e2e-output.txt 2>&1
```

Check the [Kubernetes e2e documentation](https://github.com/kubernetes/community/blob/master/contributors/devel/e2e-tests.md#end-to-end-testing-in-kubernetes) for more information about e2e tests.
