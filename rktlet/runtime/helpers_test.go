/*
Copyright 2016-2017 The Kubernetes Authors.

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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	runtimeApi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

func TestPassFilter(t *testing.T) {
	id1 := "id1"
	id2 := "id2"
	state1 := runtimeApi.ContainerState_CONTAINER_RUNNING
	state2 := runtimeApi.ContainerState_CONTAINER_EXITED
	podSandboxId1 := "podSanboxId1"
	podSandboxId2 := "podSanboxId2"
	labels1 := map[string]string{"hello": "world"}
	labels2 := map[string]string{"hello": "world", "foo": "bar"}

	tests := []struct {
		container *runtimeApi.Container
		filter    *runtimeApi.ContainerFilter
		result    bool
	}{
		// Case 0, no filters.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels1,
			},
			nil,
			true,
		},

		// Case 1, matched.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels1,
			},
			&runtimeApi.ContainerFilter{
				Id:            id1,
				State:         &runtimeApi.ContainerStateValue{state1},
				PodSandboxId:  podSandboxId1,
				LabelSelector: labels1,
			},
			true,
		},

		// Case 2, ids are not matched.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels1,
			},
			&runtimeApi.ContainerFilter{
				Id:            id2,
				State:         &runtimeApi.ContainerStateValue{state1},
				PodSandboxId:  podSandboxId1,
				LabelSelector: labels1,
			},
			false,
		},

		// Case 3, states are not matched.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels1,
			},
			&runtimeApi.ContainerFilter{
				Id:            id1,
				State:         &runtimeApi.ContainerStateValue{state2},
				PodSandboxId:  podSandboxId1,
				LabelSelector: labels1,
			},
			false,
		},

		// Case 4, pod sandbox ids are not matched.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels1,
			},
			&runtimeApi.ContainerFilter{
				Id:            id1,
				State:         &runtimeApi.ContainerStateValue{state1},
				PodSandboxId:  podSandboxId2,
				LabelSelector: labels1,
			},
			false,
		},

		// Case 5, labels are matched, superset.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels2,
			},
			&runtimeApi.ContainerFilter{
				Id:            id1,
				State:         &runtimeApi.ContainerStateValue{state1},
				PodSandboxId:  podSandboxId1,
				LabelSelector: labels1,
			},
			true,
		},

		// Case 6, labels are not matched, subset.
		{
			&runtimeApi.Container{
				Id:           id1,
				State:        state1,
				PodSandboxId: podSandboxId1,
				Labels:       labels1,
			},
			&runtimeApi.ContainerFilter{
				Id:            id1,
				State:         &runtimeApi.ContainerStateValue{state1},
				PodSandboxId:  podSandboxId1,
				LabelSelector: labels2,
			},
			false,
		},
	}

	for i, tt := range tests {
		testHint := fmt.Sprintf("test case #%d", i)
		assert.Equal(t, tt.result, passFilter(tt.container, tt.filter), testHint)
	}
}

func TestGeneratePortArgs(t *testing.T) {
	protocol := runtimeApi.Protocol_TCP
	containerPort := int32(80)
	hostPort := int32(8080)
	hostIP := "127.0.0.1"

	tests := []struct {
		port   *runtimeApi.PortMapping
		result string
	}{
		// Case 0.
		{
			&runtimeApi.PortMapping{
				Protocol:      protocol,
				ContainerPort: containerPort,
				HostPort:      hostPort,
				HostIp:        hostIP,
			},
			"--port=tcp-80-8080:tcp:80:127.0.0.1:8080",
		},

		// Case 1, empty host IP, should default to "0.0.0.0".
		{
			&runtimeApi.PortMapping{
				Protocol:      protocol,
				ContainerPort: containerPort,
				HostPort:      hostPort,
			},
			"--port=tcp-80-8080:tcp:80:0.0.0.0:8080",
		},
	}

	for i, tt := range tests {
		testHint := fmt.Sprintf("test case #%d", i)
		assert.Equal(t, tt.result, generatePortArgs(tt.port), testHint)
	}
}

func TestBuildAppName(t *testing.T) {
	tests := []struct {
		inputAttempt  uint32
		inputName     string
		resultAppName string
	}{
		// Case 0, normal lowercase characters
		{
			0,
			"containername",
			"0-containername",
		},
		// Case 1, convertion from uppercase to lowercase
		{
			0,
			"ContainerName",
			"0-containername",
		},
	}

	for i, tt := range tests {
		testHint := fmt.Sprintf("test case #%d", i)
		appName, err := buildAppName(tt.inputAttempt, tt.inputName)
		if err != nil {
			t.Errorf("error building app name from container name %s: %v\n", tt.inputName, err)
			continue
		}
		assert.Equal(t, tt.resultAppName, appName, testHint)
	}
}

func TestParseContainerID(t *testing.T) {
	tests := []struct {
		inputID       string
		resultAppName string
	}{
		// Case 0, normal lowercase characters
		{
			"01234567-89ab-cdef-0123-456789abcdef:containerid",
			"containerid",
		},
		// Case 1, convertion from uppercase to lowercase
		{
			"01234567-89ab-cdef-0123-456789abcdef:ContainerID",
			"containerid",
		},
	}

	for i, tt := range tests {
		testHint := fmt.Sprintf("test case #%d", i)
		_, appName, err := parseContainerID(tt.inputID)
		if err != nil {
			t.Errorf("error parsing container ID %s: %v\n", tt.inputID, err)
			continue
		}
		assert.Equal(t, tt.resultAppName, appName, testHint)
	}
}
