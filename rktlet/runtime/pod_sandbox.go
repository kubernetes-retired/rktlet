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
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/coreos/rkt/lib"
	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"golang.org/x/net/context"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

func formatPod(metaData *runtimeApi.PodSandboxMetadata) string {
	return fmt.Sprintf("%s_%s(%s)", metaData.Name, metaData.Namespace, metaData.Uid)
}

func (r *RktRuntime) RunPodSandbox(ctx context.Context, req *runtimeApi.RunPodSandboxRequest) (*runtimeApi.RunPodSandboxResponse, error) {
	metaData := req.GetConfig().GetMetadata()
	k8sPodUid := metaData.GetUid()
	podUUIDFile, err := ioutil.TempFile("", "rktlet_"+k8sPodUid)
	defer os.Remove(podUUIDFile.Name())
	if err != nil {
		return nil, fmt.Errorf("could not create temporary file for rkt UUID: %v", err)
	}

	// Let the init process to run the pod sandbox.
	command := generateAppSandboxCommand(req, podUUIDFile.Name())

	cmd := r.Command(command[0], command[1:]...)

	var cgroupParent string
	linux := req.GetConfig().GetLinux()
	if linux != nil {
		cgroupParent = linux.GetCgroupParent()
	}

	id, err := r.Init.StartProcess(cgroupParent, cmd[0], cmd[1:]...)
	if err != nil {
		glog.Errorf("failed to run pod %q: %v", formatPod(metaData), err)
		return nil, err

	}

	glog.V(4).Infof("pod sandbox is running as service %q", id)

	var rktUUID string
	// TODO, switch to sdnotify, possibly with a fallback for non-systemd or non-coreos stage1
	for i := 0; i < 100; i++ {
		data, err := ioutil.ReadAll(podUUIDFile)
		if err != nil {
			return nil, fmt.Errorf("error reading rkt pod UUID file: %v", err)
		}
		if len(data) != 0 {
			rktUUID = string(data)
			break
		}

		time.Sleep(100 * time.Millisecond)
	}
	if rktUUID == "" {
		return nil, fmt.Errorf("waited 10s for pod sandbox to start, but it didn't: %v", k8sPodUid)
	}

	// Wait for the status to be running too, up to 10 more seconds
	var status *runtimeApi.PodSandboxStatusResponse
	for i := 0; i < 100; i++ {
		status, err = r.PodSandboxStatus(ctx, &runtimeApi.PodSandboxStatusRequest{PodSandboxId: &rktUUID})
		if err == nil {
			break
		}
		if status.GetStatus().GetState() != runtimeApi.PodSandboxState_SANDBOX_READY {
			continue
		}
		time.Sleep(100 * time.Millisecond)
	}
	if status.GetStatus().GetState() != runtimeApi.PodSandboxState_SANDBOX_READY {
		glog.Warningf("sandbox got a UUID but did not have a ready status after 10s: %v, %v", status, err)
		return &runtimeApi.RunPodSandboxResponse{PodSandboxId: &rktUUID}, fmt.Errorf("unable to get status within 10s: %v", err)
	}

	// TODO(euank): this is a temporary hack due to https://github.com/coreos/rkt/issues/3423
	// it should be removed once that issue is resolved/released
	time.Sleep(3 * time.Second)

	// Inject internal logging app
	// TODO: This can be removed once https://github.com/coreos/rkt/pull/3396
	// handles logging
	err = r.addInternalLoggingApp(ctx, rktUUID, req.GetConfig().GetLogDirectory())

	return &runtimeApi.RunPodSandboxResponse{PodSandboxId: &rktUUID}, err
}

func (r *RktRuntime) stopPodSandbox(ctx context.Context, id string, force bool) error {
	_, err := r.RunCommand(
		"stop",
		"--force="+strconv.FormatBool(force),
		id,
	)
	if err == nil {
		return nil
	}

	if cli.RktStopIsAlreadyStoppedError(err) || cli.RktStopIsNotExistError(err) {
		glog.V(4).Infof("ignoring stop error for idempotency: %v", err)
		return nil
	}
	return err
}

func (r *RktRuntime) StopPodSandbox(ctx context.Context, req *runtimeApi.StopPodSandboxRequest) (*runtimeApi.StopPodSandboxResponse, error) {
	err := r.stopPodSandbox(ctx, req.GetPodSandboxId(), false)
	return &runtimeApi.StopPodSandboxResponse{}, err
}

func (r *RktRuntime) RemovePodSandbox(ctx context.Context, req *runtimeApi.RemovePodSandboxRequest) (*runtimeApi.RemovePodSandboxResponse, error) {
	// Force stop first, per api contract "if there are any running containers in
	// the sandbox, they must be forcibly terminated
	r.stopPodSandbox(ctx, req.GetPodSandboxId(), true)

	// Retry rm a few times to work around
	// https://github.com/kubernetes-incubator/rktlet/issues/21#issuecomment-264842608
	// TODO(euank): this won't be needed once the above is fixed
	var err error
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		_, err = r.RunCommand("rm", req.GetPodSandboxId())
		if i == maxRetries-1 || err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	return &runtimeApi.RemovePodSandboxResponse{}, err
}

func (r *RktRuntime) PodSandboxStatus(ctx context.Context, req *runtimeApi.PodSandboxStatusRequest) (*runtimeApi.PodSandboxStatusResponse, error) {
	resp, err := r.RunCommand("status", req.GetPodSandboxId(), "--format=json")
	if err != nil {
		return nil, err
	}

	if len(resp) != 1 {
		return nil, fmt.Errorf("unexpected result %q", resp)
	}

	var pod rkt.Pod
	if err := json.Unmarshal([]byte(resp[0]), &pod); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pod: %v", err)
	}

	status, err := toPodSandboxStatus(&pod)
	if err != nil {
		return nil, fmt.Errorf("error converting pod status: %v", err)
	}
	return &runtimeApi.PodSandboxStatusResponse{Status: status}, nil
}

func (r *RktRuntime) ListPodSandbox(ctx context.Context, req *runtimeApi.ListPodSandboxRequest) (*runtimeApi.ListPodSandboxResponse, error) {
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

	sandboxes := make([]*runtimeApi.PodSandbox, 0, len(pods))
	for i, _ := range pods {
		p := pods[i]
		sandboxStatus, err := toPodSandboxStatus(&p)
		if err != nil {
			return nil, fmt.Errorf("error converting the status of pod sandbox %v: %v", p.UUID, err)
		}

		if !podSandboxStatusMatchesFilter(sandboxStatus, req.GetFilter()) {
			continue
		}

		sandboxes = append(sandboxes, &runtimeApi.PodSandbox{
			Id:        sandboxStatus.Id,
			Labels:    sandboxStatus.Labels,
			Metadata:  sandboxStatus.Metadata,
			State:     sandboxStatus.State,
			CreatedAt: sandboxStatus.CreatedAt,
		})
	}

	return &runtimeApi.ListPodSandboxResponse{Items: sandboxes}, nil
}
