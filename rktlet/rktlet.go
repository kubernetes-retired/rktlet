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
	"os"

	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"github.com/kubernetes-incubator/rktlet/rktlet/image"
	"github.com/kubernetes-incubator/rktlet/rktlet/runtime"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/util/exec"
)

func New(config *Config) (ContainerAndImageService, error) {
	execer := exec.New()

	if config.RktPath == "" {
		rktPath, err := execer.LookPath("rkt")
		if err != nil {
			return nil, fmt.Errorf("must have rkt installed: %v", err)
		}
		config.RktPath = rktPath
	}

	if _, err := os.Stat(config.RktPath); err != nil {
		return nil, fmt.Errorf("rkt binary did not exist at %q: %v", config.RktPath, err)
	}

	systemdRunPath, err := execer.LookPath("systemd-run")
	if err != nil {
		return nil, fmt.Errorf("must have systemd-run installed: %v", err)
	}

	rktCli := cli.NewRktCLI(config.RktPath, execer, cli.CLIConfig{
		InsecureOptions: []string{"image", "ondisk"},
		Dir:             config.RktDatadir,
	})
	init := cli.NewSystemd(systemdRunPath, execer)

	imageStore := image.NewImageStore(image.ImageStoreConfig{CLI: rktCli})

	rktRuntime, err := runtime.New(rktCli,
		init,
		imageStore,
		config.StreamServerAddress,
		config.RktStage1Name,
		config.NetworkPluginName)
	if err != nil {
		return nil, err
	}

	return combinedRuntimes{
		RuntimeServiceServer: rktRuntime,
		ImageServiceServer:   imageStore,
	}, nil
}

type Config struct {
	RktDatadir    string
	RktPath       string
	RktStage1Name string

	// StreamServerAddress is the address the rktlet stream server should listen on.
	// This address must be accessible by the api-server. However, it also allows
	// arbitrary code execution within pods and must be secured.
	StreamServerAddress string

	NetworkPluginName string

	// TODO, podcidr, networkdir, etc for cni
}

var DefaultConfig = &Config{
	RktDatadir:          "/var/lib/rktlet/data",
	StreamServerAddress: "0.0.0.0:10241",
}

type ContainerAndImageService interface {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer
}

type combinedRuntimes struct {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer
}
