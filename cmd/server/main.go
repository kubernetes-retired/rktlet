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
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/cmd/server/options"
	"github.com/kubernetes-incubator/rktlet/rktlet"
	"github.com/kubernetes-incubator/rktlet/version"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"k8s.io/kubernetes/pkg/kubectl/util/logs"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/util/flag"
)

const defaultUnixSock = "/var/run/rktlet.sock"

func printVersion() {
	fmt.Println("rktlet version:", version.Version)
}

func main() {
	s := options.NewRktletServer()
	s.AddFlags(pflag.CommandLine)
	flag.InitFlags()
	logs.InitLogs()
	defer logs.FlushLogs()

	if s.ShowVersion {
		printVersion()
		os.Exit(0)
	}

	exitCh := make(chan os.Signal, 1)
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	socketPath := defaultUnixSock
	defer os.Remove(socketPath)

	sock, err := net.Listen("unix", socketPath)
	if err != nil {
		glog.Fatalf("Error listening on sock %q: %v ", socketPath, err)
	}
	defer sock.Close()

	grpcServer := grpc.NewServer()

	rktruntime, err := rktlet.New(s.Config)
	if err != nil {
		glog.Fatalf("could not create rktlet: %v", err)
	}

	runtimeapi.RegisterImageServiceServer(grpcServer, rktruntime)
	runtimeapi.RegisterRuntimeServiceServer(grpcServer, rktruntime)

	glog.Infof("Starting to serve on %q", socketPath)
	go grpcServer.Serve(sock)

	<-exitCh

	glog.Infof("rktlet service exiting...")
}
