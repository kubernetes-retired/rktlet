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

// CLI is an interface for interacting with the rkt command line interface
type CLI interface {
	With(CLIConfig) CLI
	RunCommand(string, ...string) ([]string, error)
	Command(string, ...string) []string
}

// Init is an interface for interacting with the init system on the host
// (e.g. systemd), to run rkt commands.
type Init interface {
	StartProcess(command string, args ...string) (id string, err error)
	// TODO(yifan): Add StopProcess?
}

//go:generate ../../hack/generate/mockery.sh . CLI ./mocks/cli.go
//go:generate ../../hack/generate/mockery.sh . Init ./mocks/init.go
