## Local cluster

* Build and start rktlet:

```shell
# In the rktlet repo's root dir.
$ make
go build -o bin/rktlet ./cmd/server/main.go

$ sudo ./bin/rktlet -v=4
...
```

By default, the `rktlet` service will listen on a unix socket `/var/run/rktlet.sock`.

* Start a local cluster and set the container runtime type == `remote`, and tell kubelet where to contact the remote runtime.

```shell
# In the Kubernetes repo's root dir.
$ export LOG_LEVEL=6 CONTAINER_RUNTIME=remote
$ export CONTAINER_RUNTIME_ENDPOINT=/var/run/rktlet.sock
$ export IMAGE_SERVICE_ENDPOINT=/var/run/rktlet.sock
$ ./hack/local-up-cluster.sh
...
To start using your cluster, open up another terminal/tab and run:

  export KUBERNETES_PROVIDER=local

  cluster/kubectl.sh config set-cluster local --server=http://127.0.0.1:8080 --insecure-skip-tls-verify=true
  cluster/kubectl.sh config set-context local --cluster=local
  cluster/kubectl.sh config use-context local
  cluster/kubectl.sh

```

* Now we are able to launch pods:

```shell
$ kubectl create -f example/pod
pod "nginx" created

$ kubectl get pods
NAME      READY     STATUS    RESTARTS   AGE
nginx     1/1       Running   0          57s
```
