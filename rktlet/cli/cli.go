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
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/golang/glog"
	utilexec "k8s.io/utils/exec"
)

var (
	errFlagTagNotFound           = errors.New("arg: given field doesn't have a `flag` tag")
	errStructFieldNotInitialized = errors.New("arg: given field is unitialized")
)

// TODO(tmrts): refactor these into an util pkg
// Uses reflection to retrieve the `flag` tag of a field.
// The value of the `flag` field with the value of the field is
// used to construct a POSIX long flag argument string.
func getLongFlagFormOfField(fieldValue reflect.Value, fieldType reflect.StructField) (string, error) {
	flagTag := fieldType.Tag.Get("flag")
	if flagTag == "" {
		return "", errFlagTagNotFound
	}

	if !fieldValue.IsValid() {
		return "", errStructFieldNotInitialized
	}

	switch fieldValue.Kind() {
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		if fieldValue.Len() == 0 {
			return "", nil
		}

		var args []string
		for i := 0; i < fieldValue.Len(); i++ {
			args = append(args, fieldValue.Index(i).String())
		}

		return fmt.Sprintf("--%v=%v", flagTag, strings.Join(args, ",")), nil
	case reflect.String:
		if fieldValue.Len() == 0 {
			return "", nil
		}
	}

	return fmt.Sprintf("--%v=%v", flagTag, fieldValue), nil
}

// Uses reflection to transform a struct containing fields with `flag` tags
// to a string slice of POSIX compliant long form arguments.
func getArgumentFormOfStruct(strt interface{}) (flags []string) {
	numberOfFields := reflect.ValueOf(strt).NumField()

	for i := 0; i < numberOfFields; i++ {
		fieldValue := reflect.ValueOf(strt).Field(i)
		fieldType := reflect.TypeOf(strt).Field(i)

		flagFormOfField, err := getLongFlagFormOfField(fieldValue, fieldType)
		if err != nil {
			continue
		}
		if flagFormOfField == "" {
			continue
		}

		flags = append(flags, flagFormOfField)
	}

	return
}

func getFlagFormOfStruct(strt interface{}) (flags []string) {
	return getArgumentFormOfStruct(strt)
}

type CLIConfig struct {
	Debug bool `flag:"debug"`

	Dir             string `flag:"dir"`
	LocalConfigDir  string `flag:"local-config"`
	UserConfigDir   string `flag:"user-config"`
	SystemConfigDir string `flag:"system-config"`

	InsecureOptions []string `flag:"insecure-options"`
}

func (cfg *CLIConfig) Merge(newCfg CLIConfig) {
	newCfgVal := reflect.ValueOf(newCfg)
	newCfgType := reflect.TypeOf(newCfg)

	numberOfFields := newCfgVal.NumField()

	for i := 0; i < numberOfFields; i++ {
		fieldValue := newCfgVal.Field(i)
		fieldType := newCfgType.Field(i)

		if !fieldValue.IsValid() {
			continue
		}

		newCfgVal.FieldByName(fieldType.Name).Set(fieldValue)
	}
}

type cli struct {
	rktPath string
	config  CLIConfig
	execer  utilexec.Interface

	globalFlags []string
}

func (c *cli) With(cfg CLIConfig) CLI {
	copyCfg := c.config

	copyCfg.Merge(cfg)

	return NewRktCLI(c.rktPath, c.execer, copyCfg)
}

// RunCommand execute a rkt command with the given subCmd and args.
func (c *cli) RunCommand(subCmd string, args ...string) ([]string, error) {
	command := c.Command(subCmd, args...)
	glog.V(4).Infof("rkt: calling cmd %v", command)
	cmd := c.execer.Command(command[0], command[1:]...)

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		glog.Warningf("rkt: cmd %v %v errored with %v, %q", subCmd, args, err, output)
		return nil, fmt.Errorf("failed to run %v %v: %v\noutput: %s", subCmd, args, err, output)
	}

	return strings.Split(strings.TrimSpace(output), "\n"), nil
}

// Command returns the final rkt command that will be executed by RunCommand.
// e.g. `rkt status --debug=true $UUID`.
func (c *cli) Command(subCmd string, args ...string) []string {
	return append(append([]string{c.rktPath, subCmd}, c.globalFlags...), args...)
}

// TODO(tmrts): implement CLI with timeout
func NewRktCLI(rktPath string, exec utilexec.Interface, cfg CLIConfig) CLI {
	// this can be removed once 'app' is stable in rkt
	if err := os.Setenv("RKT_EXPERIMENT_APP", "true"); err != nil {
		panic(err)
	}
	if err := os.Setenv("RKT_EXPERIMENT_ATTACH", "true"); err != nil {
		panic(err)
	}
	return &cli{rktPath: rktPath, config: cfg, execer: exec, globalFlags: getFlagFormOfStruct(cfg)}
}
