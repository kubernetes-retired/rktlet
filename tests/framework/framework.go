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
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime"

	"github.com/kubernetes-incubator/rktlet/rktlet"
	"golang.org/x/net/context"
)

// All test images should be listed here so they can be prepulled
var (
	TestImageBusybox = "busybox:1.25.1"
	TestImageFedora  = "fedora:25"
)

var allTestImages = []string{
	TestImageBusybox,
	TestImageFedora,
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

	imageIDs map[string]string
}

func Setup(t *testing.T) *TestContext {
	var err error
	RequireRoot(t)

	tmpDir := os.Getenv("RKTLET_TESTDIR")
	if tmpDir == "" {
		tmpDir, err = ioutil.TempDir(os.Getenv("TMPDIR"), fmt.Sprintf("rktlet_test_%d", time.Now().Unix()))
		if err != nil {
			t.Fatalf("unable to make tmpdir for test: %v", err)
		}
	}

	rktRuntime, err := rktlet.New(&rktlet.Config{
		RktDatadir:          filepath.Join(tmpDir, "rkt_data"),
		StreamServerAddress: "127.0.0.1:0", // :0 so multiple setup instances don't conflict
	})
	if err != nil {
		t.Fatalf("unable to initialize rktlet: %v", err)
	}

	imageIDs := make(map[string]string)

	for _, image := range allTestImages {
		// TODO, retry this N times
		image := image
		imageRef, err := rktRuntime.PullImage(context.TODO(), &runtime.PullImageRequest{
			Image: &runtime.ImageSpec{
				Image: image,
			},
		})
		if err != nil {
			t.Fatalf("error pulling image %q for test: %v", image, err)
		}
		imageIDs[image] = imageRef.ImageRef
	}

	return &TestContext{
		T:      t,
		Rktlet: rktRuntime,
		TmpDir: tmpDir,
		LogDir: filepath.Join(tmpDir, "cri_logs"),

		imageIDs: imageIDs,
	}
}

func (t *TestContext) ImageRef(key string) string {
	ref, ok := t.imageIDs[key]
	if !ok {
		panic(fmt.Sprintf("unable to resolve image ref: %v", key))
	}
	return ref
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
		t.Logf("removing pod %q", sandbox.Id)
		_, err := t.Rktlet.RemovePodSandbox(context.Background(), &runtime.RemovePodSandboxRequest{
			PodSandboxId: sandbox.Id,
		})
		if err != nil {
			t.Errorf("error removing pod sandbox %q: %v", sandbox.Id, err)
		}
	}

	tmpDir := os.Getenv("RKTLET_TESTDIR")
	if tmpDir == "" {
		os.RemoveAll(t.TmpDir)
	}
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
			Uid:       uid,
			Attempt:   attempt,
			Name:      podName,
			Namespace: namespace,
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
	config.LogDirectory = pod.LogDir

	resp, err := t.Rktlet.RunPodSandbox(context.Background(), &runtime.RunPodSandboxRequest{
		Config: config,
	})

	if err != nil {
		t.Fatalf("unexpected error running pod %s: %v", name, err)
	}
	sboxId := resp.PodSandboxId
	if sboxId == "" {
		t.Fatalf("empty sandbox ID returned for %s", name)
	}
	pod.SandboxId = sboxId

	return pod
}

// RunContainerToExit runs a container and returns its exit code
func (p *Pod) RunContainerToExit(ctx context.Context, cfg *runtime.ContainerConfig) (string, int32) {
	resp, err := p.t.Rktlet.CreateContainer(ctx, &runtime.CreateContainerRequest{
		PodSandboxId: p.SandboxId,
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
		statusResp, err = p.t.Rktlet.ContainerStatus(ctx, &runtime.ContainerStatusRequest{
			ContainerId: resp.ContainerId,
		})
		if err != nil {
			p.t.Fatalf("error getting container status in %v: %v", p.Name, err)
		}
		if statusResp.GetStatus().State == runtime.ContainerState_CONTAINER_EXITED {
			break
		}
		p.t.Logf("waiting more for status; currently have %v", statusResp.GetStatus().State)
		time.Sleep(1 * time.Second)
	}

	// TODO: removed when this was de-pointered
	/*
		if statusResp.GetStatus().ExitCode == nil {
			p.t.Fatalf("expected status to have exit code set after exiting in %v: %+v", p.Name, statusResp.GetStatus())
		} */

	// This is a hack to dodge a slight race between a container exiting and
	// readlogs. The time it takes journal2cri to convert the journald output of
	// a container to the cri log format is non-zero, so this is to wait for the output to be ready for being read
	time.Sleep(1 * time.Second)

	var stdout, stderr bytes.Buffer
	err = kuberuntime.ReadLogs(logPath(p.t.LogDir, p.Metadata.Uid, cfg.Metadata.Name, cfg.Metadata.Attempt), &v1.PodLogOptions{}, &stdout, &stderr)
	if err != nil {
		// Hack warning! Work around https://github.com/kubernetes-incubator/rktlet/issues/88
		// ReadLogs wraps errors so os.IsNotExist doesn't work
		if !strings.Contains(err.Error(), "no such file or directory") {
			p.t.Errorf("unable to get logs for pod %v", err)
		}
	}

	// merge stdout/stderr
	stdout.Write(stderr.Bytes())
	return stdout.String(), statusResp.GetStatus().ExitCode
}

func logPath(root string, uid string, name string, attempt uint32) string {
	// https://github.com/kubernetes/kubernetes/blob/b5cf713bc73db6d94f78a2c6cd49ef981c0c80dd/pkg/kubelet/kuberuntime/helpers.go#L209-L222
	containerLogsPath := fmt.Sprintf("%s_%d.log", name, attempt)
	podLogsDir := filepath.Join(root, uid)
	return filepath.Join(podLogsDir, containerLogsPath)
}

// WaitStable waits for the given pod to be in a 'running' state and for at
// least numContainers of its containers to likewise be running
func (p *Pod) WaitStable(ctx context.Context, numContainers int) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for stable pod: %v", ctx.Err())
		default:
		}

		sandboxStatus, err := p.t.Rktlet.PodSandboxStatus(ctx, &runtime.PodSandboxStatusRequest{
			PodSandboxId: p.SandboxId,
		})
		if err != nil {
			return err
		}
		if sandboxStatus.GetStatus().State != runtime.PodSandboxState_SANDBOX_READY {
			time.Sleep(1 * time.Second)
			continue
		}

		containers, err := p.t.Rktlet.ListContainers(ctx, &runtime.ListContainersRequest{
			Filter: &runtime.ContainerFilter{
				PodSandboxId: p.SandboxId,
			},
		})
		if err != nil {
			return err
		}
		numRunning := 0
		for _, container := range containers.Containers {
			if container.State == runtime.ContainerState_CONTAINER_RUNNING {
				numRunning++
			}
		}
		if numRunning == numContainers {
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}

func (p *Pod) ContainerID(ctx context.Context, name string) (string, error) {
	containers, err := p.t.Rktlet.ListContainers(ctx, &runtime.ListContainersRequest{
		Filter: &runtime.ContainerFilter{
			PodSandboxId: p.SandboxId,
		},
	})
	if err != nil {
		return "", err
	}
	for _, container := range containers.GetContainers() {
		if container.GetMetadata().Name == name {
			return container.Id, nil
		}
	}
	return "", fmt.Errorf("could not find container named %v in pod", name)
}
