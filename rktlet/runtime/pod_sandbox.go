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

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	rkt "github.com/rkt/rkt/api/v1"
	"golang.org/x/net/context"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func formatPod(metaData *runtimeApi.PodSandboxMetadata) string {
	return fmt.Sprintf("%s_%s(%s)", metaData.Name, metaData.Namespace, metaData.Uid)
}

func (r *RktRuntime) RunPodSandbox(ctx context.Context, req *runtimeApi.RunPodSandboxRequest) (*runtimeApi.RunPodSandboxResponse, error) {
	metaData := req.GetConfig().GetMetadata()
	k8sPodUid := metaData.Uid
	podUUIDFile, err := ioutil.TempFile("", "rktlet_"+k8sPodUid)
	defer os.Remove(podUUIDFile.Name())
	if err != nil {
		return nil, fmt.Errorf("could not create temporary file for rkt UUID: %v", err)
	}

	// Let the init process to run the pod sandbox.
	command, err := generateAppSandboxCommand(req, podUUIDFile.Name(), r.stage1Name, r.networkPluginName)
	if err != nil {
		return nil, err
	}

	cmd := r.Command(command[0], command[1:]...)

	var cgroupParent string
	linux := req.GetConfig().GetLinux()
	if linux != nil {
		cgroupParent = linux.CgroupParent
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

	statusResp, err := r.PodSandboxStatus(ctx, &runtimeApi.PodSandboxStatusRequest{PodSandboxId: rktUUID})
	if err != nil {
		return &runtimeApi.RunPodSandboxResponse{PodSandboxId: rktUUID}, fmt.Errorf("unable to get status: %v", err)
	}

	if statusResp.Status.State != runtimeApi.PodSandboxState_SANDBOX_READY {
		return &runtimeApi.RunPodSandboxResponse{PodSandboxId: rktUUID}, fmt.Errorf("sandbox timeout: %v", err)
	}

	return &runtimeApi.RunPodSandboxResponse{PodSandboxId: rktUUID}, err
}

func (r *RktRuntime) stopPodSandbox(ctx context.Context, id string, force bool) error {
	_, err := r.RunCommand(
		"stop",
		"--force="+strconv.FormatBool(force),
		id,
	)

	if err != nil {
		if !cli.RktStopIsAlreadyStoppedError(err) && !cli.RktStopIsNotExistError(err) {
			return err
		}

		glog.V(4).Infof("ignoring stop error for idempotency: %v", err)
	}

	if _, err := r.PodSandboxStatus(ctx, &runtimeApi.PodSandboxStatusRequest{PodSandboxId: id}); err != nil {
		return err
	}

	return nil
}

func (r *RktRuntime) StopPodSandbox(ctx context.Context, req *runtimeApi.StopPodSandboxRequest) (*runtimeApi.StopPodSandboxResponse, error) {
	err := r.stopPodSandbox(ctx, req.PodSandboxId, false)
	return &runtimeApi.StopPodSandboxResponse{}, err
}

func (r *RktRuntime) RemovePodSandbox(ctx context.Context, req *runtimeApi.RemovePodSandboxRequest) (*runtimeApi.RemovePodSandboxResponse, error) {
	// Force stop first, per api contract "if there are any running containers in
	// the sandbox, they must be forcibly terminated
	r.stopPodSandbox(ctx, req.PodSandboxId, true)

	_, err := r.RunCommand("rm", req.PodSandboxId)

	return &runtimeApi.RemovePodSandboxResponse{}, err
}

func (r *RktRuntime) PodSandboxStatus(ctx context.Context, req *runtimeApi.PodSandboxStatusRequest) (*runtimeApi.PodSandboxStatusResponse, error) {
	resp, err := r.RunCommand("status", req.PodSandboxId, "--format=json", "--wait-ready=10s")
	if err != nil {
		glog.Warningf("sandbox got a UUID but did not have a ready status after 10s: %v", err)

		// the pod wasn't ready after 10s, try to get its status so we can
		// return meaningful data to the kubelet
		resp, err = r.RunCommand("status", req.PodSandboxId, "--format=json")
		if err != nil {
			return nil, err
		}
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
		if !isKubernetesPod(&p) {
			glog.V(6).Infof("Skipping non-kubernetes pod %s", p.UUID)
			continue
		}
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
