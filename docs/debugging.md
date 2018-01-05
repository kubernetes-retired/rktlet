# Debugging

This guide describes ways to debug pods not working correctly, from using Kubernetes tools to debugging rkt itself.

## Kubernetes tools

If the cause of the error is in the app itself, you should see some hints in the logs:

```
$ kubectl logs POD [CONTAINER]
```

It might be useful to get a shell into the app to debug it further:

```
$ kubectl exec -it POD sh
```

Note that for this to work the container image needs to have some kind of shell.

For more information about debugging pods using kubernetes tools, check the [kubernetes documentation][k8s-debug-pods].

## Debugging rktlet

It might happen that your pod keeps restarting and you see errors like these when running `kubectl describe POD`:

```
  7s		7s		1	kubelet, kubespawndefault1			Warning		FailedSync	Error syncing pod, skipping: failed to "CreatePodSandbox" for "simple_default(53820056-5a7b-11e7-8046-c85b763781a4)" with CreatePodSandboxError: "CreatePodSandbox for pod \"simple_default(53820056-5a7b-11e7-8046-c85b763781a4)\" failed: rpc error: code = 2 desc = unable to get status within 10s: <nil>"
```

This means the pod sandbox couldn't start, but we don't get any useful information on the reason why that happened.

To get more information we can check the rktlet logs for the node the pod was scheduled on.
First, we need to figure out what node is that:

```
$ kubectl describe POD | grep Node
Node:           kubespawndefault1/10.22.0.4
```

Then we can ssh into the node and check the rktlet logs:

```
$ journalctl -u rktlet
```

We'll see something like this:

```
I0626 16:26:16.194978    9604 pod_sandbox.go:65] pod sandbox is running as service "rktlet-c173629f-cdd1-41f1-a3ef-b14d00e2ce63"
...
W0626 16:26:16.332112    9604 pod_sandbox.go:98] sandbox got a UUID but did not have a ready status after 10s: status:<id:"c536dbb9-6451-42fe-8a48-fc038cccef90" metadata:<name:"simple" uid:"53820056-5a7b-11e7-8046-c85b763781a4" namespace:"default" > state:SANDBOX_NOTREADY network:<> labels:<key:"io.kubernetes.pod.name" value:"simple" > labels:<key:"io.kubernetes.pod.namespace" value:"default" > labels:<key:"io.kubernetes.pod.uid" value:"53820056-5a7b-11e7-8046-c85b763781a4" > labels:<key:"name" value:"simple" > annotations:<key:"kubernetes.io/config.seen" value:"2017-06-26T16:25:38.667546831+02:00" > annotations:<key:"kubernetes.io/config.source" value:"api" > > , <nil>
```

Notice the `pod sandbox is running as service "rktlet-c173629f-cdd1-41f1-a3ef-b14d00e2ce63"` message.
This is the systemd service rktlet used to start the rkt pod.

Let's check its logs:

```
$ journalctl -u rktlet-c173629f-cdd1-41f1-a3ef-b14d00e2ce63
-- Logs begin at Fri 2017-03-17 11:27:56 CET, end at Mon 2017-06-26 16:32:50 CEST. --
Jun 26 16:26:16 neptune systemd[1]: Started /bin/rkt app --debug=false --dir=/var/lib/rktlet/data --insecure-options=image,ondisk sandbox --uuid-file-save=/tmp/rktlet_53820056-5
Jun 26 16:26:16 neptune rkt[22729]: sandbox: error with dns flags: no other --dns options allowed when --dns=host is passed
Jun 26 16:26:16 neptune systemd[1]: rktlet-c173629f-cdd1-41f1-a3ef-b14d00e2ce63.service: Main process exited, code=exited, status=1/FAILURE
Jun 26 16:26:16 neptune systemd[1]: rktlet-c173629f-cdd1-41f1-a3ef-b14d00e2ce63.service: Unit entered failed state.
Jun 26 16:26:16 neptune systemd[1]: rktlet-c173629f-cdd1-41f1-a3ef-b14d00e2ce63.service: Failed with result 'exit-code'.
```

Here we can see the actual error: `no other --dns options allowed when --dns=host is passed`.

This process is very cumbersome. Ideally, the error would be reported to the kubelet and we would see it in `kubectl describe POD`.
[kubernetes-incubator/rktlet#108](https://github.com/kubernetes-incubator/rktlet/issues/108) tracks this feature.

## Debugging rkt

Sometimes it's useful to interact with rkt itself to debug a problem.
By default, rktlet uses `/var/lib/rktlet/data` as rkt data dir, so to interact with containers created by rktlet all rkt commands should include `--dir=/var/lib/rktlet/data`.

Some useful commands to run are:

```
# get the list of rkt pods
$ rkt --dir=/var/lib/rktlet/data list

# enter a rkt pod
$ rkt --dir=/var/lib/rktlet/data enter UUID

# get a pod manifest to tweak it later with different parameters
$ rkt --dir=/var/lib/rktlet/data cat-manifest UUID > pod.json
```

Another useful thing to do is [get the logs of failed pods][rkt-stopped-logs].

Check [rkt's debugging guide][rkt-debugging] for more details on debugging rkt.

[k8s-debug-pods]: https://kubernetes.io/docs/tasks/debug-application-cluster/debug-pod-replication-controller/
[rkt-debugging]: https://github.com/rkt/rkt/tree/master/Documentation/devel/debugging.md
[rkt-stopped-logs]: https://github.com/rkt/rkt/blob/v1.29.0/Documentation/commands.md#stopped-pod
