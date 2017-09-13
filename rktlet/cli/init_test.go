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

package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCgroupParentToSliceName(t *testing.T) {

	testCases := []struct {
		cgroupParent string
		sliceName    string
		failed       bool
	}{
		{
			cgroupParent: "/kubepods.slice/kubepods-guaranteed.slice/kubepods-guaranteed-pod5c5979ec_9871_11e7_b58f_c85b763781a4.slice",
			sliceName:    "kubepods-guaranteed-pod5c5979ec_9871_11e7_b58f_c85b763781a4.slice",
			failed:       false,
		},
		{
			cgroupParent: "/kubepods/besteffort/pod5c5979ec-9871-11e7-b58f-c85b763781a4",
			sliceName:    "",
			failed:       true,
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			sliceName, err := cgroupParentToSliceName(testCase.cgroupParent)
			assert.Equal(t, testCase.failed, err != nil)
			assert.Equal(t, testCase.sliceName, sliceName)
		})
	}

}
