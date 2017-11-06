/*
Copyright 2017 The Kubernetes Authors.

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

package util

import (
	"testing"
)

func TestCanonicalImageName(t *testing.T) {
	for i, tt := range []struct {
		inName       string
		expectedName string
	}{
		{
			"busybox",
			"docker://docker.io/library/busybox:latest",
		},
		{
			"docker://busybox",
			"docker://docker.io/library/busybox:latest",
		},
		{
			"busybox:latest",
			"docker://busybox:latest",
		},
		{
			"docker://busybox:latest",
			"docker://busybox:latest",
		},
		{
			"sha512-72b96529483a709900dae6277618028651ef338702dcaa361fa705884c85181a",
			"sha512-72b96529483a709900dae6277618028651ef338702dcaa361fa705884c85181a",
		},
	} {
		outName, err := GetCanonicalImageName(tt.inName)
		if err != nil {
			t.Errorf("GetCanonicalImageName %d returned error: %v", i, err)
		}

		if tt.expectedName != outName {
			t.Errorf("GetCanonicalImageName %d expected %v got %v: %v", i, tt.expectedName, outName, err)
		}
	}
}
