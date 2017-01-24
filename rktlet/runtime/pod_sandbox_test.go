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
	"testing"

	rktlib "github.com/coreos/rkt/lib"
	"github.com/coreos/rkt/networking/netinfo"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli/mocks"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

func strptr(s string) *string    { return &s }
func int64ptr(i int64) *int64    { return &i }
func uint32ptr(i uint32) *uint32 { return &i }

var podsandboxReady = runtime.PodSandboxState_SANDBOX_READY
var podsandboxNotReady = runtime.PodSandboxState_SANDBOX_NOTREADY

func TestListPodSandbox(t *testing.T) {
	testCases := []struct {
		Filter   *runtime.PodSandboxFilter
		RktPods  []rktlib.Pod
		Response runtime.ListPodSandboxResponse
	}{
		{ // simple case of 1 pod
			Filter: nil,
			RktPods: []rktlib.Pod{{
				UUID:      "1",
				AppNames:  []string{"0-foo", "1-bar"},
				Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
				StartedAt: int64ptr(100),
				UserAnnotations: map[string]string{
					kubernetesReservedAnnoPodName:      "foo",
					kubernetesReservedAnnoPodUid:       "0",
					kubernetesReservedAnnoPodAttempt:   "0",
					kubernetesReservedAnnoPodNamespace: "default",
				},
				State: "running",
			}},
			Response: runtime.ListPodSandboxResponse{
				Items: []*runtime.PodSandbox{{
					Id:        "1",
					CreatedAt: 100,
					State:     podsandboxReady,
					Metadata: &runtime.PodSandboxMetadata{
						Name:      "foo",
						Attempt:   0,
						Namespace: "default",
						Uid:       "0",
					},
				}},
			},
		},
		{ // 3 pods, 2 running
			Filter: nil,
			RktPods: []rktlib.Pod{
				{
					UUID:      "1",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(100),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo",
						kubernetesReservedAnnoPodUid:       "0",
						kubernetesReservedAnnoPodAttempt:   "0",
						kubernetesReservedAnnoPodNamespace: "default",
					},
					State: "running",
				},
				{
					UUID:      "2",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(102),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo2",
						kubernetesReservedAnnoPodUid:       "10",
						kubernetesReservedAnnoPodAttempt:   "5",
						kubernetesReservedAnnoPodNamespace: "not-default",
					},
					State: "running",
				},
				{
					UUID:      "3",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(104),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo3",
						kubernetesReservedAnnoPodUid:       "0",
						kubernetesReservedAnnoPodAttempt:   "0",
						kubernetesReservedAnnoPodNamespace: "default",
					},
					State: "stopped",
				},
			},
			Response: runtime.ListPodSandboxResponse{
				Items: []*runtime.PodSandbox{
					{
						Id:        "1",
						CreatedAt: 100,
						State:     podsandboxReady,
						Metadata: &runtime.PodSandboxMetadata{
							Name:      "foo",
							Attempt:   0,
							Namespace: "default",
							Uid:       "0",
						},
					},
					{
						Id:        "2",
						CreatedAt: 102,
						State:     podsandboxReady,
						Metadata: &runtime.PodSandboxMetadata{
							Name:      "foo2",
							Attempt:   5,
							Namespace: "not-default",
							Uid:       "10",
						},
					},
					{
						Id:        "3",
						CreatedAt: 104,
						State:     podsandboxNotReady,
						Metadata: &runtime.PodSandboxMetadata{
							Name:      "foo3",
							Attempt:   0,
							Namespace: "default",
							Uid:       "0",
						},
					},
				},
			},
		},
		{ // filter for one pod by id
			Filter: &runtime.PodSandboxFilter{
				Id: "1",
			},
			RktPods: []rktlib.Pod{
				{
					UUID:      "1",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(100),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo",
						kubernetesReservedAnnoPodUid:       "0",
						kubernetesReservedAnnoPodAttempt:   "0",
						kubernetesReservedAnnoPodNamespace: "default",
					},
					State: "running",
				},
				{
					UUID:      "2",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(102),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo2",
						kubernetesReservedAnnoPodUid:       "10",
						kubernetesReservedAnnoPodAttempt:   "5",
						kubernetesReservedAnnoPodNamespace: "not-default",
					},
					State: "running",
				},
			},
			Response: runtime.ListPodSandboxResponse{
				Items: []*runtime.PodSandbox{
					{
						Id:        "1",
						CreatedAt: 100,
						State:     podsandboxReady,
						Metadata: &runtime.PodSandboxMetadata{
							Name:      "foo",
							Attempt:   0,
							Namespace: "default",
							Uid:       "0",
						},
					},
				},
			},
		},
		{ // Simple filter for one pod by state
			Filter: &runtime.PodSandboxFilter{
				State: &runtime.PodSandboxStateValue{podsandboxReady},
			},
			RktPods: []rktlib.Pod{
				{
					UUID:      "1",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(100),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo",
						kubernetesReservedAnnoPodUid:       "0",
						kubernetesReservedAnnoPodAttempt:   "0",
						kubernetesReservedAnnoPodNamespace: "default",
					},
					State: "running",
				},
				{
					UUID:      "2",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(102),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo2",
						kubernetesReservedAnnoPodUid:       "10",
						kubernetesReservedAnnoPodAttempt:   "5",
						kubernetesReservedAnnoPodNamespace: "not-default",
					},
					State: "stopped",
				},
			},
			Response: runtime.ListPodSandboxResponse{
				Items: []*runtime.PodSandbox{
					{
						Id:        "1",
						CreatedAt: 100,
						State:     podsandboxReady,
						Metadata: &runtime.PodSandboxMetadata{
							Name:      "foo",
							Attempt:   0,
							Namespace: "default",
							Uid:       "0",
						},
					},
				},
			},
		},
		{ // Simple filter for one pod by labels
			Filter: &runtime.PodSandboxFilter{
				LabelSelector: map[string]string{"foo": "bar"},
			},
			RktPods: []rktlib.Pod{
				{
					UUID:      "1",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(100),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo",
						kubernetesReservedAnnoPodUid:       "0",
						kubernetesReservedAnnoPodAttempt:   "0",
						kubernetesReservedAnnoPodNamespace: "default",
					},
					UserLabels: map[string]string{
						"foo": "bar",
					},
					State: "running",
				},
				{
					UUID:      "2",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 1}}},
					StartedAt: int64ptr(102),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo2",
						kubernetesReservedAnnoPodUid:       "10",
						kubernetesReservedAnnoPodAttempt:   "5",
						kubernetesReservedAnnoPodNamespace: "not-default",
					},
					UserLabels: map[string]string{
						"foo": "baz",
					},
					State: "stopped",
				},
				{
					UUID:      "3",
					AppNames:  []string{"0-foo", "1-bar"},
					Networks:  []netinfo.NetInfo{{NetName: "default", IP: []byte{10, 0, 0, 3}}},
					StartedAt: int64ptr(103),
					UserAnnotations: map[string]string{
						kubernetesReservedAnnoPodName:      "foo3",
						kubernetesReservedAnnoPodUid:       "10",
						kubernetesReservedAnnoPodAttempt:   "5",
						kubernetesReservedAnnoPodNamespace: "not-default",
					},
					State: "running",
				},
			},
			Response: runtime.ListPodSandboxResponse{
				Items: []*runtime.PodSandbox{
					{
						Id:        "1",
						CreatedAt: 100,
						State:     podsandboxReady,
						Metadata: &runtime.PodSandboxMetadata{
							Name:      "foo",
							Attempt:   0,
							Namespace: "default",
							Uid:       "0",
						},
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
		},
	}

	for i, testCase := range testCases {
		mockCli := new(mocks.CLI)
		mockRuntime := &RktRuntime{
			CLI: mockCli,
		}

		rktpodJson, err := json.Marshal(testCase.RktPods)
		if err != nil {
			t.Fatalf("%d: could not marshal input: %v", i, err)
		}
		mockCli.On("RunCommand", "list", []string{"--format=json"}).Return([]string{string(rktpodJson)}, nil)

		resp, err := mockRuntime.ListPodSandbox(context.TODO(), &runtime.ListPodSandboxRequest{
			Filter: testCase.Filter,
		})
		if err != nil {
			t.Errorf("%d: error listing pod sandbox: %v", i, err)
			continue
		}

		assert.Equal(t, testCase.Response, *resp)
		mockCli.AssertExpectations(t)
	}
}
