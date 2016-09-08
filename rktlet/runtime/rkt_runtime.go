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

package runtime

import (
	"fmt"

	"github.com/kubernetes-incubator/rktlet/rktlet/cli"

	context "golang.org/x/net/context"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

type RktRuntime struct {
	cli cli.CLI
}

// NewImageStore creates an image storage that allows CRUD operations for images.
func New(cli cli.CLI) runtimeApi.RuntimeServiceServer {
	return &RktRuntime{
		cli: cli,
	}
}

func (r *RktRuntime) Version(ctx context.Context, req *runtimeApi.VersionRequest) (*runtimeApi.VersionResponse, error) {
	name := "rkt"
	version := "0.1.0"
	return &runtimeApi.VersionResponse{
		Version:           &version, // kubelet/remote version, must be 0.1.0
		RuntimeName:       &name,
		RuntimeVersion:    &version, // todo, rkt version
		RuntimeApiVersion: &version, // todo, rkt version
	}, nil
}

func (r *RktRuntime) ContainerStatus(ctx context.Context, req *runtimeApi.ContainerStatusRequest) (*runtimeApi.ContainerStatusResponse, error) {
	return nil, fmt.Errorf("TODO")
}

func (r *RktRuntime) CreateContainer(ctx context.Context, req *runtimeApi.CreateContainerRequest) (*runtimeApi.CreateContainerResponse, error) {
	return nil, fmt.Errorf("TODO")
}

func (r *RktRuntime) StartContainer(ctx context.Context, req *runtimeApi.StartContainerRequest) (*runtimeApi.StartContainerResponse, error) {
	return nil, fmt.Errorf("TODO")
}

func (r *RktRuntime) StopContainer(ctx context.Context, req *runtimeApi.StopContainerRequest) (*runtimeApi.StopContainerResponse, error) {
	return nil, fmt.Errorf("TODO")
}

func (r *RktRuntime) ListContainers(ctx context.Context, req *runtimeApi.ListContainersRequest) (*runtimeApi.ListContainersResponse, error) {
	return &runtimeApi.ListContainersResponse{}, nil
}

func (r *RktRuntime) RemoveContainer(ctx context.Context, req *runtimeApi.RemoveContainerRequest) (*runtimeApi.RemoveContainerResponse, error) {
	return nil, fmt.Errorf("TODO")
}
