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
	"io"
	"os/exec"

	"github.com/golang/glog"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

func (r *RktRuntime) Exec(execService runtimeApi.RuntimeService_ExecServer) error {
	// Read the 1st request, which contains container ID, commands, stdin.
	req, err := execService.Recv()
	if err != nil {
		return err
	}

	uuid, appName, err := parseContainerID(*req.ContainerId)
	if err != nil {
		return err
	}

	// Since "k8s.io/kubernetes/pkg/util/exec.Cmd" doesn't include
	// StdinPipe(), StdoutPipe() and StderrPipe() in the interface,
	// so we have to use the "Cmd" under "os/exec" package.
	// TODO(yifan): Patch upstream to include SetStderr() in the interface.
	cmdList := []string{"app", "exec", "--app=" + appName, uuid}
	cmdList = append(cmdList, req.Cmd...)
	rktCommand := r.Command(cmdList[0], cmdList[1:]...)
	cmd := exec.Command(rktCommand[0], rktCommand[1:]...)

	// At most one error will happen in each of the following goroutines.
	errCh := make(chan error, 4)
	done := make(chan struct{})

	go streamStdin(cmd, execService, errCh)
	go streamStdout(cmd, execService, errCh)
	go streamStderr(cmd, execService, errCh)
	go run(cmd, errCh, done)

	select {
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

func streamStdin(cmd *exec.Cmd, execService runtimeApi.RuntimeService_ExecServer, errCh chan error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		errCh <- err
		return
	}

	for {
		req, err := execService.Recv()
		if err != nil {
			glog.Errorf("rkt: Error receiving request: %v", err)
			errCh <- err
			return
		}

		// Write wil return an error if it stops early before
		// finishing writing.
		if _, err := stdin.Write(req.Stdin); err != nil {
			glog.Errorf("rkt: Error writing to stdin: %v", err)
			errCh <- err
			return
		}
	}
}

func streamStdout(cmd *exec.Cmd, execService runtimeApi.RuntimeService_ExecServer, errCh chan error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		errCh <- err
		return
	}

	b := make([]byte, 1024)

	for {
		n, err := stdout.Read(b)
		if err == io.EOF {
			return
		}
		if err != nil {
			glog.Errorf("rkt: Error reading stdout: %v", err)
			errCh <- err
			return
		}

		if err := execService.Send(&runtimeApi.ExecResponse{Stdout: b[:n]}); err != nil {
			glog.Errorf("rkt: Error sending exec response for stdout: %v", err)
			errCh <- err
			return
		}
	}
}

func streamStderr(cmd *exec.Cmd, execService runtimeApi.RuntimeService_ExecServer, errCh chan error) {
	stderr, err := cmd.StderrPipe()
	if err != nil {
		errCh <- err
		return
	}

	b := make([]byte, 1024)

	for {
		n, err := stderr.Read(b)
		if err == io.EOF {
			return
		}
		if err != nil {
			glog.Errorf("rkt: Error reading stdout: %v", err)
			errCh <- err
			return
		}

		if err := execService.Send(&runtimeApi.ExecResponse{Stderr: b[:n]}); err != nil {
			glog.Errorf("rkt: Error sending exec response for stderr: %v", err)
			errCh <- err
			return
		}
	}
}

func run(cmd *exec.Cmd, errCh chan error, done chan struct{}) {
	if err := cmd.Start(); err != nil {
		errCh <- err
		return
	}
	if err := cmd.Wait(); err != nil {
		errCh <- err
		return
	}
	close(done)
	return
}
