package runtime_integ_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
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
			Image: strptr(tc.ImageRef(framework.TestImageBusybox)),
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

func TestPrivileged(t *testing.T) {
	tc := framework.Setup(t)
	defer tc.Teardown()

	podName := "test_privileged"
	p := tc.RunPod(podName, &runtime.PodSandboxConfig{
		Linux: &runtime.LinuxPodSandboxConfig{
			SecurityContext: &runtime.LinuxSandboxSecurityContext{
				Privileged: boolptr(true),
			},
		},
	})

	// These cases are based on this list: https://github.com/kubernetes/kubernetes/blob/f49442d4331052c3141c47a3f9701da7082ebcff/pkg/kubelet/api/v1alpha1/runtime/api.proto#L462-L470
	privilegedCases := []struct {
		Name          string
		Command       string
		ShouldContain []string
	}{
		// 1: caps
		{
			Name:          "capabilities",
			Command:       "capsh --print",
			ShouldContain: []string{"cap_sys_admin", "cap_net_admin"},
		},
		// 2: no path masking
		{
			Name:          "unmasked-sysfs",
			Command:       `ls /sys/fs/cgroup && echo success`,
			ShouldContain: []string{"success"},
		},
		// 3. RW sysfs and proc. TODO, currently rkt does not support this
		// $ touch /proc/sys/vm/panic_on_oom
		// 4. Apparmor: N/A for rkt
		// 5. Seccomp
		{
			Name:          "seccomp",
			Command:       "mount -o remount / && echo success",
			ShouldContain: []string{"success"},
		},
		// 6: device cgroup: TODO, currently rkt does not support this
		// 7: All devices from the host: TODO, currently rkt does not support this
		// 8: No selinux applied: TODO, though rkt should support this one as-is
	}

	for i, testCase := range privilegedCases {
		t.Logf("Checking for %q on pod %q", testCase.Name, podName)
		runConfig := &runtime.ContainerConfig{
			Image: &runtime.ImageSpec{
				Image: strptr(tc.ImageRef(framework.TestImageFedora)),
			},
			Command: []string{"sh", "-c", testCase.Command},
			Metadata: &runtime.ContainerMetadata{
				Name:    strptr(testCase.Name),
				Attempt: uint32ptr(uint32(i)),
			},
			Linux: &runtime.LinuxContainerConfig{
				SecurityContext: &runtime.LinuxContainerSecurityContext{
					Privileged: boolptr(true),
				},
			},
		}
		output, exitCode := p.RunContainerToExit(context.TODO(), runConfig)
		if exitCode != 0 {
			t.Fatalf("%s: expected %d, got %d: %v", testCase.Name, 0, exitCode, output)
		}

		for _, el := range testCase.ShouldContain {
			assert.Contains(t, output, el)
		}
	}
}

func TestExecSync(t *testing.T) {
	tc := framework.Setup(t)
	defer tc.Teardown()

	p := tc.RunPod("test_execsync", &runtime.PodSandboxConfig{
		Linux: &runtime.LinuxPodSandboxConfig{
			SecurityContext: &runtime.LinuxSandboxSecurityContext{
				Privileged: boolptr(true),
			},
		},
	})

	containerDone := &sync.WaitGroup{}
	containerDone.Add(1)
	name := "testexecsync"

	go func() {
		runConfig := &runtime.ContainerConfig{
			Image: &runtime.ImageSpec{
				Image: strptr(tc.ImageRef(framework.TestImageBusybox)),
			},
			Command: []string{"sh", "-c", `while true; do if [ -f /exit ]; then exit $(cat /exit); fi; sleep 1; done`},
			Metadata: &runtime.ContainerMetadata{
				Name:    &name,
				Attempt: uint32ptr(0),
			},
		}
		_, exitCode := p.RunContainerToExit(context.TODO(), runConfig)
		assert.Equal(t, int32(42), exitCode)
		containerDone.Done()
	}()

	err := p.WaitStable(context.TODO(), 1)
	assert.Nil(t, err, "could not wait stable")
	containerID, err := p.ContainerID(context.TODO(), name)
	assert.Nil(t, err, "could not get container ID")
	fmt.Printf("got containerID %s\n", containerID)

	execRes, err := tc.Rktlet.ExecSync(context.TODO(), &runtime.ExecSyncRequest{
		ContainerId: &containerID,
		Cmd:         []string{"sh", "-c", "echo 42 > /exit; echo success; sleep 0.5"},
	})
	assert.Nil(t, err)
	assert.Equal(t, int32(0), execRes.GetExitCode())
	assert.Equal(t, "success", strings.TrimSpace(string(execRes.GetStdout())))

	containerDone.Wait()
}
