package runtime_integ_test

import (
	"context"
	"io/ioutil"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	"github.com/kubernetes-incubator/rktlet/tests/framework"
	"github.com/stretchr/testify/assert"
)

func int64ptr(i int64) *int64    { return &i }
func uint32ptr(i uint32) *uint32 { return &i }
func strptr(s string) *string    { return &s }
func boolptr(b bool) *bool       { return &b }

func TestHostNetwork(t *testing.T) {
	hostsEtcHosts, err := ioutil.ReadFile("/etc/hosts")
	if err != nil {
		t.Fatalf("could not get hosts' /etc/hosts file: %v", err)
	}

	tc := framework.Setup(t)
	defer tc.Teardown()

	p := tc.RunPod("test_hostnetwork", &runtime.PodSandboxConfig{
		Linux: &runtime.LinuxPodSandboxConfig{
			SecurityContext: &runtime.LinuxSandboxSecurityContext{
				NamespaceOptions: &runtime.NamespaceOption{
					HostNetwork: boolptr(true),
				},
			},
		},
	})

	// Test the container's output is what we expect
	runConfig := &runtime.ContainerConfig{
		Image: &runtime.ImageSpec{
			Image: strptr("busybox:1.25.1"),
		},
		Command: []string{"sh", "-c", `cat /etc/hosts`},
		Metadata: &runtime.ContainerMetadata{
			Name:    strptr("etchosts"),
			Attempt: uint32ptr(0),
		},
	}
	output, exitCode := p.RunContainerToExit(context.TODO(), runConfig)
	if exitCode != 0 {
		t.Fatalf("expected %d, got %d: %v", 0, exitCode, output)
	}

	// Due to https://github.com/coreos/rkt/issues/3473 we need to trim spaces for each line
	assert.Equal(t, trimLines(string(hostsEtcHosts)), trimLines(output))
}

// trimeLines trims any blank lines and removes leading/trailing whitespace
func trimLines(lines string) string {
	parts := strings.Split(strings.TrimSpace(lines), "\n")

	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return strings.Join(result, "\n")
}
