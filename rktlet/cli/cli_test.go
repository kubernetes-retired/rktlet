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

package cli

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFlagFormOfStruct(t *testing.T) {

	testCases := []struct {
		in  CLIConfig
		out []string
	}{
		{
			in: CLIConfig{
				Debug: true,
			},
			out: []string{"--debug=true"},
		},
		{
			in: CLIConfig{
				Debug: true,
				Dir:   "foo",
			},
			out: []string{"--debug=true", "--dir=foo"},
		},
		{
			in: CLIConfig{
				Debug:           true,
				InsecureOptions: []string{"all", "all-run"},
			},
			out: []string{"--debug=true", "--insecure-options=all,all-run"},
		},
	}

	for i, testCase := range testCases {
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			flags := getFlagFormOfStruct(testCase.in)
			assert.Equal(t, testCase.out, flags)
		})
	}

}
