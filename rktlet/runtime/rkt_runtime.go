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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/rkt/lib"
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"golang.org/x/net/context"

	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

type RktRuntime struct {
	cli.CLI
	cli.Init
}

// NewImageStore creates an image storage that allows CRUD operations for images.
func New(cli cli.CLI, init cli.Init) runtimeApi.RuntimeServiceServer {
	return &RktRuntime{cli, init}
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
	// Container ID is in the form of "uuid:appName".
	uuid, appName, err := parseContainerID(*req.ContainerId)
	if err != nil {
		return nil, err
	}

	resp, err := r.RunCommand("app", "status", uuid, "--app="+appName, "--format=json")
	if err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fmt.Errorf("unexpected result %q", resp)
	}

	var app rkt.App
	if err := json.Unmarshal([]byte(resp[0]), &app); err != nil {
		return nil, fmt.Errorf("failed to unmarshal container: %v", err)
	}

	status, err := toContainerStatus(uuid, &app)
	if err != nil {
		return nil, fmt.Errorf("failed to convert to container status: %v", err)
	}
	return &runtimeApi.ContainerStatusResponse{Status: status}, nil
}

func (r *RktRuntime) CreateContainer(ctx context.Context, req *runtimeApi.CreateContainerRequest) (*runtimeApi.CreateContainerResponse, error) {
	// TODO(yifan): For now, let's just assume the podsandbox config is not used.
	// TODO(yifan): More fields need to be supported by 'rkt app add'.

	var imageID string

	// Get the image hash.
	imageName := *req.Config.Image.Image
	resp, err := r.RunCommand("image", "fetch", "--store-only=true", "--full=true", "docker://"+imageName)
	if err != nil {
		return nil, err
	}
	for _, line := range resp {
		if strings.HasPrefix(line, "sha512") {
			imageID = line
			break
		}
	}

	if imageID == "" {
		return nil, fmt.Errorf("failed to get image ID for image %q", imageName)
	}

	command := generateAppAddCommand(req, imageID)
	if _, err := r.RunCommand(command[0], command[1:]...); err != nil {
		return nil, err
	}

	appName := buildAppName(*req.Config.Metadata.Attempt, *req.Config.Metadata.Name)
	containerID := buildContainerID(*req.PodSandboxId, appName)

	return &runtimeApi.CreateContainerResponse{ContainerId: &containerID}, nil
}

func (r *RktRuntime) StartContainer(ctx context.Context, req *runtimeApi.StartContainerRequest) (*runtimeApi.StartContainerResponse, error) {
	// Container ID is in the form of "uuid:appName".
	uuid, appName, err := parseContainerID(*req.ContainerId)
	if err != nil {
		return nil, err
	}

	if _, err := r.RunCommand("app", "start", uuid, "--app="+appName); err != nil {
		return nil, err
	}
	return &runtimeApi.StartContainerResponse{}, nil
}

func (r *RktRuntime) StopContainer(ctx context.Context, req *runtimeApi.StopContainerRequest) (*runtimeApi.StopContainerResponse, error) {
	// Container ID is in the form of "uuid:appName".
	uuid, appName, err := parseContainerID(*req.ContainerId)
	if err != nil {
		return nil, err
	}

	// TODO(yifan): Support timeout.
	if _, err := r.RunCommand("app", "stop", uuid, "--app="+appName); err != nil {
		return nil, err
	}
	return &runtimeApi.StopContainerResponse{}, nil
}

func (r *RktRuntime) ListContainers(ctx context.Context, req *runtimeApi.ListContainersRequest) (*runtimeApi.ListContainersResponse, error) {
	// We assume the containers in data dir are all managed by kubelet.
	resp, err := r.RunCommand("list", "--format=json")
	if err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fmt.Errorf("unexpected result %q", resp)
	}

	var pods []rkt.Pod
	if err := json.Unmarshal([]byte(resp[0]), &pods); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pods: %v", err)
	}

	// TODO(yifan): Could optimize this so that we don't have to check ContainerStatus on every container.
	var containers []*runtimeApi.Container
	for _, p := range pods {
		for _, appName := range p.AppNames {
			containerID := buildContainerID(p.UUID, appName)
			resp, err := r.ContainerStatus(ctx, &runtimeApi.ContainerStatusRequest{
				ContainerId: &containerID,
			})
			if err != nil {
				glog.Warningf("rkt: cannot get container status for pod %q, app %q: %v", p.UUID, appName, err)
				continue
			}
			// TODO: filter.
			containers = append(containers, &runtimeApi.Container{
				Id:          resp.Status.Id,
				Metadata:    resp.Status.Metadata,
				Image:       resp.Status.Image,
				ImageRef:    resp.Status.ImageRef,
				State:       resp.Status.State,
				Labels:      resp.Status.Labels,
				Annotations: resp.Status.Annotations,
			})
		}
	}

	return &runtimeApi.ListContainersResponse{Containers: containers}, nil
}

func (r *RktRuntime) RemoveContainer(ctx context.Context, req *runtimeApi.RemoveContainerRequest) (*runtimeApi.RemoveContainerResponse, error) {
	// Container ID is in the form of "uuid:appName".
	uuid, appName, err := parseContainerID(*req.ContainerId)
	if err != nil {
		return nil, err
	}

	// TODO(yifan): Support timeout.
	if _, err := r.RunCommand("app", "rm", uuid, "--app="+appName); err != nil {
		return nil, err
	}
	return &runtimeApi.RemoveContainerResponse{}, nil
}
