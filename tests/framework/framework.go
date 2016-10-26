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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	"github.com/kubernetes-incubator/rktlet/rktlet"
	"golang.org/x/net/context"
)

func RequireRoot(t *testing.T) {
	if os.Getuid() != 0 {
		t.Fatalf("test requires root")
	}
}

type TestContext struct {
	*testing.T
	Rktlet rktlet.ContainerAndImageService
	TmpDir string
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

	return &TestContext{
		T:      t,
		Rktlet: rktRuntime,
		TmpDir: tmpDir,
	}
}

func (t *TestContext) Teardown() {
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

	if t.Failed() {
		t.Logf("leaving tempdir for failed test: %v", t.TmpDir)
	} else {
		os.RemoveAll(t.TmpDir)
	}
}
