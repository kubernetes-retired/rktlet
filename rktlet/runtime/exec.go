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
	"bytes"
	"errors"
	"io"
	"os/exec"
	"syscall"

	"github.com/golang/glog"
	"github.com/kr/pty"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	utilexec "k8s.io/utils/exec"

	"golang.org/x/net/context"

	"k8s.io/client-go/tools/remotecommand"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
	"k8s.io/kubernetes/pkg/kubelet/server/streaming"
	"k8s.io/kubernetes/pkg/kubelet/util/ioutils"
	"k8s.io/kubernetes/pkg/util/term"
)

func (r *RktRuntime) Attach(ctx context.Context, req *runtimeapi.AttachRequest) (*runtimeapi.AttachResponse, error) {
	return r.streamServer.GetAttach(req)
}

func (r *RktRuntime) Exec(ctx context.Context, req *runtimeapi.ExecRequest) (*runtimeapi.ExecResponse, error) {
	return r.streamServer.GetExec(req)
}

func (r *RktRuntime) ExecSync(ctx context.Context, req *runtimeapi.ExecSyncRequest) (*runtimeapi.ExecSyncResponse, error) {
	var stdout, stderr bytes.Buffer

	// TODO: Respect req.Timeout
	exitCode := int32(0)
	err := r.execShim.Exec(req.ContainerId, req.Cmd, nil, ioutils.WriteCloserWrapper(&stdout), ioutils.WriteCloserWrapper(&stderr), false, nil)
	exitErr, ok := err.(utilexec.ExitError)
	if ok {
		exitCode = int32(exitErr.ExitStatus())
	}

	// rktlet internal error
	if !ok && err != nil {
		return nil, err
	}

	return &runtimeapi.ExecSyncResponse{
		ExitCode: exitCode,
		Stderr:   stderr.Bytes(),
		Stdout:   stdout.Bytes(),
	}, nil
}

func (r *RktRuntime) PortForward(ctx context.Context, req *runtimeapi.PortForwardRequest) (*runtimeapi.PortForwardResponse, error) {
	return r.streamServer.GetPortForward(req)
}

type execShim struct {
	cli cli.CLI
}

var _ streaming.Runtime = &execShim{}

func NewExecShim(cli cli.CLI) *execShim {
	return &execShim{cli: cli}
}

func (es *execShim) Attach(containerID string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	return errors.New("TODO")
}

// Exec executes a given command in a container
func (es *execShim) Exec(containerID string, cmd []string, in io.Reader, out, errOut io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	uuid, appName, err := parseContainerID(containerID)
	if err != nil {
		return err
	}

	// TODO(euank): Make it possible to use "k8s.io/kubernetes/pkg/util/exec.Cmd"
	// by adding more methods (for mocking)
	cmdList := []string{"app", "exec", "--app=" + appName, uuid}
	cmdList = append(cmdList, cmd...)
	rktCommand := es.cli.Command(cmdList[0], cmdList[1:]...)
	execCmd := exec.Command(rktCommand[0], rktCommand[1:]...)
	glog.V(5).Infof("executing command: %v", execCmd.Args)

	if tty {
		return execWithTty(execCmd, in, out, resize)
	}

	if in != nil {
		execCmd.Stdin = in
	}
	execCmd.Stdout = out
	execCmd.Stderr = errOut

	if err := execCmd.Start(); err != nil {
		glog.Warningf("error running exec: %v", err)
		return err
	}

	if err := execCmd.Wait(); err != nil {
		glog.Warningf("error waiting for exec: %v", err)
		return newRktExitError(err)
	}
	return nil
}

func execWithTty(execCmd *exec.Cmd, in io.Reader, out io.WriteCloser, resize <-chan remotecommand.TerminalSize) error {
	p, err := pty.Start(execCmd) // calls execCmd.Start
	if err != nil {
		return err
	}
	defer p.Close()
	defer out.Close()

	// defensive check in case the resize stream isn't closed
	done := make(chan struct{}, 0)
	defer close(done)

	go func() {
		for {
			select {
			case rsz := <-resize:
				term.SetSize(p.Fd(), rsz)
			case <-done:
				return
			}
		}
	}()

	if in != nil {
		go io.Copy(p, in)
	}
	if out != nil {
		go io.Copy(out, p)
	}
	// stderr + tty can't happen

	return newRktExitError(execCmd.Wait())
}

func (es *execShim) PortForward(sandboxID string, port int32, stream io.ReadWriteCloser) error {
	return errors.New("TODO")
}

// rktExitError implements k8s.io/kubernetes/pkg/util/exec.ExitError interface.
// TODO(euank): Figure out if this actually works correctly in this impl.
type rktExitError struct{ *exec.ExitError }

var _ utilexec.ExitError = &rktExitError{}

func (r *rktExitError) ExitStatus() int {
	if status, ok := r.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return 0
}

func newRktExitError(e error) error {
	if exitErr, ok := e.(*exec.ExitError); ok {
		return &rktExitError{exitErr}
	}
	return e
}
