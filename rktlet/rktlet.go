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

// Package rktlet provides high-level functions to instantiate and use the
// rktlet CRI runtime.
package rktlet

import (
	"fmt"

	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"github.com/kubernetes-incubator/rktlet/rktlet/image"
	"github.com/kubernetes-incubator/rktlet/rktlet/runtime"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/util/exec"
)

type ContainerAndImageService interface {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer
}

type combinedRuntimes struct {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer
}

func New() (ContainerAndImageService, error) {
	execer := exec.New()
	rktPath, err := execer.LookPath("rkt")
	if err != nil {
		return nil, fmt.Errorf("must have rkt installed: %v", err)
	}

	systemdRunPath, err := execer.LookPath("systemd-run")
	if err != nil {
		return nil, fmt.Errorf("must have systemd-run installed: %v", err)
	}

	cli, init := cli.NewRktCLI(rktPath, execer, cli.CLIConfig{}), cli.NewSystemd(systemdRunPath, execer)

	return combinedRuntimes{
		RuntimeServiceServer: runtime.New(cli, init),
		ImageServiceServer:   image.NewImageStore(image.ImageStoreConfig{CLI: cli}),
	}, nil
}
