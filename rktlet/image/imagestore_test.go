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

package image

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"

	"github.com/kubernetes-incubator/rktlet/rktlet/cli/mocks"
	"github.com/stretchr/testify/mock"
)

const mockBusyboxFetchResponse = `
image: remote fetching from URL "docker://busybox"
Downloading sha256:8ddc19f1652 [===============================] 668 KB / 668 KB
sha512-847812d9cc2dd9e8bab70f7b77f8efe2`

func TestPullImage(t *testing.T) {
	mockCli := new(mocks.CLI)
	testImage := "busybox"

	mockImageStore := NewImageStore(ImageStoreConfig{CLI: mockCli, RequestTimeout: 0 * time.Second})

	mockCli.On("RunCommand", "image", mock.AnythingOfType("[]string")).Run(func(args mock.Arguments) {
		cmdArgs, ok := args.Get(1).([]string)
		if !ok {
			t.Fatalf("Expected type []string, got type %v", reflect.TypeOf(args.Get(1)))
		}
		subCommand := cmdArgs[0]
		image := cmdArgs[len(cmdArgs)-1]

		if subCommand != "fetch" {
			t.Fatalf("Expected runCommand to be a fetch command; was %v", subCommand)
		}
		if image != "docker://"+testImage {
			t.Fatalf("Expected rkt fetch to be fore image %v, was %+v", testImage, image)
		}
	}).Return(strings.Split(mockBusyboxFetchResponse, "\n"), nil)

	_, err := mockImageStore.PullImage(context.Background(), &runtime.PullImageRequest{
		Image: &runtime.ImageSpec{
			Image: &testImage,
		},
	})
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	mockCli.AssertExpectations(t)
}
