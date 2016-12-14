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

// Package framework has utility functions and helpers for running integration
// rktlet tests
package framework

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime"

	"github.com/kubernetes-incubator/rktlet/rktlet"
	"golang.org/x/net/context"
)

// All test images should be listed here so they can be prepulled
var (
	TestImageBusybox = "busybox:1.25.1"
)

var allTestImages = []string{
	TestImageBusybox,
}

func RequireRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Fatalf("test requires root")
	}
}

type TestContext struct {
	*testing.T
	Rktlet rktlet.ContainerAndImageService
	TmpDir string
	LogDir string
}

func Setup(t *testing.T) *TestContext {
	RequireRoot(t)

	tmpDir, err := ioutil.TempDir("", fmt.Sprintf("rktlet_test_%d", time.Now().Unix()))
	if err != nil {
		t.Fatalf("unable to make tmpdir for test: %v", err)
	}

	rktRuntime, err := rktlet.New(&rktlet.Config{
		RktDatadir: filepath.Join(tmpDir, "rkt_data"),
	})
	if err != nil {
		t.Fatalf("unable to initialize rktlet: %v", err)
	}

	for _, image := range allTestImages {
		// TODO, retry this N times
		image := image
		_, err := rktRuntime.PullImage(context.TODO(), &runtime.PullImageRequest{
			Image: &runtime.ImageSpec{
				Image: &image,
			},
		})
		if err != nil {
			t.Fatalf("error pulling image %q for test: %v", image, err)
		}
	}

	return &TestContext{
		T:      t,
		Rktlet: rktRuntime,
		TmpDir: tmpDir,
		LogDir: filepath.Join(tmpDir, "cri_logs"),
	}
}

func (t *TestContext) Teardown() {
	if t.Failed() {
		t.Logf("leaving tempdir for failed test: %v", t.TmpDir)
		return
	}
	allSandboxes, err := t.Rktlet.ListPodSandbox(context.Background(), &runtime.ListPodSandboxRequest{})
	if err != nil {
		t.Fatalf("expected to list sandboxes, got back: %v", err)
	}

	for i, _ := range allSandboxes.Items {
		sandbox := allSandboxes.Items[i]
		t.Logf("removing pod %q", sandbox.GetId())
		_, err := t.Rktlet.RemovePodSandbox(context.Background(), &runtime.RemovePodSandboxRequest{
			PodSandboxId: sandbox.Id,
		})
		if err != nil {
			t.Errorf("error removing pod sandbox %q: %v", sandbox.GetId(), err)
		}
	}

	os.RemoveAll(t.TmpDir)
}

func (t *TestContext) PullImages() {
}

type Pod struct {
	Name      string
	SandboxId string
	Metadata  *runtime.PodSandboxMetadata
	LogDir    string

	t *TestContext
}

// RunPod runs a pod for a test of the given name. The provided 'partialConfig'
// is optional and essential fields will be set (or overridden)
func (t *TestContext) RunPod(name string, partialConfig *runtime.PodSandboxConfig) *Pod {
	attempt := uint32(0)

	uid := fmt.Sprintf("uid_%s_%d", name, rand.Int())
	podName := fmt.Sprintf("name_%s_%d", name, rand.Int())
	namespace := fmt.Sprintf("namespace_%s_%d", name, rand.Int())
	logDir := filepath.Join(t.LogDir, uid)
	os.MkdirAll(logDir, 0777)

	pod := &Pod{
		Name: name,
		Metadata: &runtime.PodSandboxMetadata{
			Uid:       &uid,
			Attempt:   &attempt,
			Name:      &podName,
			Namespace: &namespace,
		},
		LogDir: logDir,
		t:      t,
	}

	config := &runtime.PodSandboxConfig{}
	if partialConfig != nil {
		config = partialConfig
	}
	// Override things that we need to control
	config.Metadata = pod.Metadata
	config.LogDirectory = &pod.LogDir

	resp, err := t.Rktlet.RunPodSandbox(context.Background(), &runtime.RunPodSandboxRequest{
		Config: config,
	})

	if err != nil {
		t.Fatalf("unexpected error running pod %s: %v", name, err)
	}
	sboxId := resp.GetPodSandboxId()
	if sboxId == "" {
		t.Fatalf("empty sandbox ID returned for %s", name)
	}
	pod.SandboxId = sboxId

	return pod
}

// RunContainerToExit runs a container and returns its exit code
func (p *Pod) RunContainerToExit(ctx context.Context, cfg *runtime.ContainerConfig) (string, int32) {
	resp, err := p.t.Rktlet.CreateContainer(ctx, &runtime.CreateContainerRequest{
		PodSandboxId: &p.SandboxId,
		Config:       cfg,
	})
	if err != nil {
		p.t.Fatalf("unable to create container in %v: %v", p.Name, err)
	}
	_, err = p.t.Rktlet.StartContainer(ctx, &runtime.StartContainerRequest{
		ContainerId: resp.ContainerId,
	})
	if err != nil {
		p.t.Fatalf("unable to start container in %v: %v", p.Name, err)
	}

	// Wait for it to finish running
	var statusResp *runtime.ContainerStatusResponse
	for {
		time.Sleep(1 * time.Second)
		statusResp, err = p.t.Rktlet.ContainerStatus(ctx, &runtime.ContainerStatusRequest{
			ContainerId: resp.ContainerId,
		})
		if err != nil {
			p.t.Fatalf("error getting container status in %v: %v", p.Name, err)
		}
		if statusResp.GetStatus().GetState() == runtime.ContainerState_CONTAINER_EXITED {
			break
		}
	}

	if statusResp.GetStatus().ExitCode == nil {
		p.t.Fatalf("expected status to have exit code set after exiting in %v: %+v", p.Name, statusResp.GetStatus())
	}

	var stdout, stderr bytes.Buffer
	err = kuberuntime.ReadLogs(logPath(p.t.LogDir, *p.Metadata.Uid, *cfg.Metadata.Name, *cfg.Metadata.Attempt), &api.PodLogOptions{}, &stdout, &stderr)
	if err != nil {
		p.t.Errorf("unable to get logs for pod %v", err)
	}

	// merge stdout/stderr
	stdout.Write(stderr.Bytes())
	return stdout.String(), statusResp.GetStatus().GetExitCode()
}

func logPath(root string, uid string, name string, attempt uint32) string {
	// https://github.com/kubernetes/kubernetes/blob/b5cf713bc73db6d94f78a2c6cd49ef981c0c80dd/pkg/kubelet/kuberuntime/helpers.go#L209-L222
	containerLogsPath := fmt.Sprintf("%s_%d.log", name, attempt)
	podLogsDir := filepath.Join(root, uid)
	return filepath.Join(podLogsDir, containerLogsPath)
}
