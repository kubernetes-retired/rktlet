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
	"path"
	"strings"

	"github.com/golang/glog"
	"github.com/pborman/uuid"
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

// cgroupParentToSliceName converts a cgroup path such as:
//   /kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod5c5979ec_9871_11e7_b58f_c85b763781a4.slice
// into a systemd slice name such as:
//   kubepods-besteffort-pod5c5979ec_9871_11e7_b58f_c85b763781a4.slice
//
// The systemd slice name must observe the following rules:
// - the name does not start with a "/" (otherwise that's interpreted as a mount unit)
// - the dashes ("-") are representing subdirectories
// - the name finishes with ".slice"
//
// The Kubelet must be started with --cgroup-driver=systemd
// (CGROUP_DRIVER=systemd in hack/local-up-cluster.sh), otherwise the
// cgroupParent will not be convertible.
func cgroupParentToSliceName(cgroupParent string) (string, error) {
	// Example for podBase: "kubepods-besteffort-pod5c5979ec_9871_11e7_b58f_c85b763781a4.slice"
	podBase := path.Base(cgroupParent)

	if !strings.HasSuffix(podBase, ".slice") {
		return "", fmt.Errorf("cgroup %q not convertible to slice name: please start the Kubelet with --cgroup-driver=systemd", cgroupParent)
	}

	return podBase, nil
}

// StartProcess runs the 'command + args' as a child of the init process,
// and returns the id of the process.
func (s *systemd) StartProcess(cgroupParent, command string, args ...string) (id string, err error) {
	unitName := fmt.Sprintf("rktlet-%s", uuid.New())

	cmdList := []string{s.systemdRunPath, "--unit=" + unitName, "--setenv=RKT_EXPERIMENT_APP=true", "--service-type=notify"}
	if cgroupParent != "" {
		// If cgroupParent doesn't exist in some of the subsystems,
		// it will be created (e.g. systemd, memeory, cpu). Otherwise
		// the process will be put inside them.
		slice, err := cgroupParentToSliceName(cgroupParent)
		if err != nil {
			glog.Warningf("%v", err)
			return "", err
		}
		cmdList = append(cmdList, "--slice="+slice)
	}
	cmdList = append(cmdList, command)
	cmdList = append(cmdList, args...)

	glog.V(4).Infof("Running %s", strings.Join(cmdList, " "))

	cmd := s.execer.Command(cmdList[0], cmdList[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		glog.Warningf("rkt: %v errored with %v", cmdList, err)
		return "", fmt.Errorf("failed to run systemd-run %v %v: %v\noutput: %s", command, args, err, out)
	}
	return unitName, nil
}
