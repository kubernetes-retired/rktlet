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
	"errors"

	"golang.org/x/net/context"

	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

func (r *RktRuntime) Exec(ctx context.Context, req *runtimeApi.ExecRequest) (*runtimeApi.ExecResponse, error) {
	return nil, errors.New("TODO")
}

func (r *RktRuntime) ExecSync(ctx context.Context, req *runtimeApi.ExecSyncRequest) (*runtimeApi.ExecSyncResponse, error) {
	return nil, errors.New("TODO")
}

func (r *RktRuntime) Attach(ctx context.Context, req *runtimeApi.AttachRequest) (*runtimeApi.AttachResponse, error) {
	return nil, errors.New("TODO")
}

func (r *RktRuntime) PortForward(ctx context.Context, req *runtimeApi.PortForwardRequest) (*runtimeApi.PortForwardResponse, error) {
	return nil, errors.New("TODO")
}
