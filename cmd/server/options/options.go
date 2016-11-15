/*
Copyright 2015 The Kubernetes Authors.

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

// Package options contains all of primary arguments for a rktlet
package options

import (
	"github.com/kubernetes-incubator/rktlet/rktlet"
	"github.com/spf13/pflag"
)

type RktletServer struct {
	*rktlet.Config
}

func NewRktletServer() *RktletServer {
	config := rktlet.DefaultConfig
	return &RktletServer{
		Config: config,
	}
}

func (s *RktletServer) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.RktPath, "rkt-path", s.RktPath, "Path of rkt binary. Leave empty to use the first rkt in $PATH.")
	fs.StringVar(&s.RktDatadir, "rkt-data-dir", s.RktPath, "Path to rkt's data directory. Defaults to '/var/lib/rktlet/data'.")
}
