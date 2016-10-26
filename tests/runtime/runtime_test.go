package runtime_integ_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kubernetes-incubator/rktlet/tests/framework"

	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

func TestCreatesPodSandbox(t *testing.T) {
	tc := framework.Setup(t)
	defer tc.Teardown()

	attempt := uint32(0)
	uid := "testuid"
	name := "testname"
	def := "default"
	logDir := filepath.Join(tc.TmpDir, "cri_logs", uid)
	os.MkdirAll(logDir, 0777)
	resp, err := tc.Rktlet.RunPodSandbox(context.Background(), &runtime.RunPodSandboxRequest{
		Config: &runtime.PodSandboxConfig{
			Metadata: &runtime.PodSandboxMetadata{
				Uid:       &uid,
				Attempt:   &attempt,
				Name:      &name,
				Namespace: &def,
			},
			LogDirectory: &logDir,
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	if resp.GetPodSandboxId() == "" {
		t.Errorf("Expected pod sandbox id to be nonempty")
	}
}
