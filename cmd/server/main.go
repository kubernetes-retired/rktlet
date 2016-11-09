/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"net"
	"os"
	"os/signal"
	"syscall"

	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/util/exec"

	"google.golang.org/grpc"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"github.com/kubernetes-incubator/rktlet/rktlet/image"
	"github.com/kubernetes-incubator/rktlet/rktlet/runtime"
)

const defaultUnixSock = "/var/run/rktlet.sock"

func main() {
	flag.Parse()
	glog.Warning("This rkt CRI server implementation is for development use only; we recommend using the copy of this code included in the kubelet")

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	socketPath := defaultUnixSock
	defer os.Remove(socketPath)

	sock, err := net.Listen("unix", socketPath)
	if err != nil {
		glog.Fatalf("Error listening on sock %q: %v ", socketPath, err)
	}
	defer sock.Close()

	execer := exec.New()
	os.Setenv("RKT_EXPERIMENT_APP", "true")
	rktPath, err := execer.LookPath("rkt")
	if err != nil {
		glog.Fatalf("Must have rkt installed: %v", err)
	}

	systemdRunPath, err := execer.LookPath("systemd-run")
	if err != nil {
		glog.Fatalf("Must have systemd-run installed: %v", err)
	}

	cli, init := cli.NewRktCLI(rktPath, execer, cli.CLIConfig{}), cli.NewSystemd(systemdRunPath, execer)

	grpcServer := grpc.NewServer()

	runtimeApi.RegisterImageServiceServer(grpcServer, image.NewImageStore(image.ImageStoreConfig{CLI: cli}))
	runtimeApi.RegisterRuntimeServiceServer(grpcServer, runtime.New(cli, init))

	glog.Infof("Starting to serve on %q", socketPath)
	go grpcServer.Serve(sock)

	<-exitCh

	glog.Infof("rktlet service exiting...")
}
