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
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kubernetes-incubator/rktlet/rktlet/cli/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/net/context"

	runtime "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const mockBusyboxFetchResponse = `
image: remote fetching from URL "docker://busybox:latest"
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
		if image != "docker://"+testImage+":latest" {
			t.Fatalf("Expected rkt fetch to be for image %v, was %+v", testImage, image)
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

func TestPassFilter(t *testing.T) {
	name1 := "example-image:latest"
	name2 := "example-image:old"

	tests := []struct {
		image  *runtime.Image
		filter *runtime.ImageFilter
		result bool
	}{
		// Case 0, no filters.
		{
			&runtime.Image{RepoTags: []string{name1}},
			nil,
			true,
		},

		// Case 1, empty filter.
		{
			&runtime.Image{RepoTags: []string{name1}},
			&runtime.ImageFilter{},
			true,
		},

		// Case 2, matched.
		{
			&runtime.Image{RepoTags: []string{name1}},
			&runtime.ImageFilter{Image: &runtime.ImageSpec{Image: &name1}},
			true,
		},

		// Case 3, not matched.
		{
			&runtime.Image{RepoTags: []string{name1}},
			&runtime.ImageFilter{Image: &runtime.ImageSpec{}},
			false,
		},

		// Case 4, not matched.
		{
			&runtime.Image{RepoTags: []string{name1}},
			&runtime.ImageFilter{Image: &runtime.ImageSpec{Image: &name2}},
			false,
		},
	}

	for i, tt := range tests {
		testHint := fmt.Sprintf("test case #%d", i)
		assert.Equal(t, tt.result, passFilter(tt.image, tt.filter), testHint)
	}
}
