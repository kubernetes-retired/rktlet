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
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
)

type RktRuntime struct {
	cli.CLI
	cli.Init

	execShim     *execShim
	streamServer streaming.Server
	imageStore   runtimeApi.ImageServiceServer
}

const internalAppPrefix = "rktletinternal-"

// New creates a new RuntimeServiceServer backed by rkt
func New(cli cli.CLI, init cli.Init, imageStore runtimeApi.ImageServiceServer, streamServerAddr string) (runtimeApi.RuntimeServiceServer, error) {
	runtime := &RktRuntime{
		CLI:        cli,
		Init:       init,
		imageStore: imageStore,
		execShim:   NewExecShim(cli),
	}

	var err error
	streamConfig := streaming.DefaultConfig
	streamConfig.Addr = streamServerAddr
	runtime.streamServer, err = streaming.NewServer(streamConfig, runtime.execShim)
	if err != nil {
		return nil, err
	}
	go func() {
		// TODO, runtime.streamServer.Stop() for SIGTERM or any other clean
		// shutdown of rktlet
		glog.Infof("listening for execs on: %v", streamConfig.Addr)
		err := runtime.streamServer.Start(true)
		if err != nil {
			glog.Fatalf("error serving execs: %v", err)
		}
	}()
	err = runtime.initializeLoggingAppImage(context.TODO())
	return runtime, err
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
	imageID := req.GetConfig().GetImage().GetImage()

	command, err := generateAppAddCommand(req, imageID)
	if err != nil {
		return nil, err
	}
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
		p := p
		for _, appName := range p.AppNames {
			if strings.HasPrefix(appName, internalAppPrefix) {
				continue
			}
			containerID := buildContainerID(p.UUID, appName)
			resp, err := r.ContainerStatus(ctx, &runtimeApi.ContainerStatusRequest{
				ContainerId: &containerID,
			})
			if err != nil {
				glog.Warningf("rkt: cannot get container status for pod %q, app %q: %v", p.UUID, appName, err)
				continue
			}

			container := &runtimeApi.Container{
				Annotations:  resp.Status.Annotations,
				CreatedAt:    resp.Status.CreatedAt,
				Id:           resp.Status.Id,
				Image:        resp.Status.Image,
				ImageRef:     resp.Status.ImageRef,
				Labels:       resp.Status.Labels,
				Metadata:     resp.Status.Metadata,
				PodSandboxId: &p.UUID,
				State:        resp.Status.State,
			}

			if passFilter(container, req.Filter) {
				containers = append(containers, container)
			}
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

func (r *RktRuntime) UpdateRuntimeConfig(ctx context.Context, req *runtimeApi.UpdateRuntimeConfigRequest) (*runtimeApi.UpdateRuntimeConfigResponse, error) {
	// TODO, use the PodCIDR passed in once we have network plugins setup
	return &runtimeApi.UpdateRuntimeConfigResponse{}, nil
}

func (r *RktRuntime) Status(ctx context.Context, req *runtimeApi.StatusRequest) (*runtimeApi.StatusResponse, error) {
	// TODO: implement

	//Need to copy the consts to get pointers
	runtimeReady := runtimeApi.RuntimeReady
	networkReady := runtimeApi.NetworkReady
	tv := true

	conditions := []*runtimeApi.RuntimeCondition{
		&runtimeApi.RuntimeCondition{
			Type:   &runtimeReady,
			Status: &tv,
		},
		&runtimeApi.RuntimeCondition{
			Type:   &networkReady,
			Status: &tv,
		},
	}
	resp := runtimeApi.StatusResponse{
		Status: &runtimeApi.RuntimeStatus{
			Conditions: conditions,
		},
	}

	return &resp, nil
}
