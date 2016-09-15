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
	"regexp"
	"strings"

	"github.com/golang/glog"
	utilexec "k8s.io/kubernetes/pkg/util/exec"
)

type systemd struct {
	systemdRunPath string
	execer         utilexec.Interface
}

// NewSystemd creates an Init object with the path to `systemd-run`.
func NewSystemd(systemdRunPath string, execer utilexec.Interface) Init {
	return &systemd{systemdRunPath, execer}
}

// StartProcess runs the 'command + args' as a child of the init process,
// and returns the id of the process.
func (s *systemd) StartProcess(command string, args ...string) (id string, err error) {
	cmd := s.execer.Command(s.systemdRunPath, append([]string{command}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.Warningf("rkt: systemd-run %v %v errored with %v", command, args, err)
		return "", fmt.Errorf("failed to run systemd-run %v %v: %v\noutput: %s", command, args, err, out)
	}

	// The output is like 'Running as unit run-30192.service.'
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(line, "Running as unit") {
			re := regexp.MustCompile(`run-[\d]+\.service`)
			serviceName := re.FindString(line)
			if serviceName != "" {
				return serviceName, nil
			}
		}
	}
	return "", fmt.Errorf("failed to find the service name of the process")
}
