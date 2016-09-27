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
	"fmt"
	"strconv"
	"strings"

	"github.com/coreos/rkt/lib"
	"github.com/coreos/rkt/networking/netinfo"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	// Exists per app.
	kubernetesReservedAnnoImageNameKey = "k8s.io/reserved/image-name"

	// Exists per pod.
	kubernetesReservedAnnoPodUid       = "k8s.io/reserved/pod-uid"
	kubernetesReservedAnnoPodName      = "k8s.io/reserved/pod-name"
	kubernetesReservedAnnoPodNamespace = "k8s.io/reserved/pod-namespace"
	kubernetesReservedAnnoPodAttempt   = "k8s.io/reserved/pod-attempt"
)

// List of reserved keys in the annotations.
var kubernetesReservedAnnoKeys []string

func init() {
	kubernetesReservedAnnoKeys = []string{
		kubernetesReservedAnnoImageNameKey,
		kubernetesReservedAnnoPodUid,
		kubernetesReservedAnnoPodName,
		kubernetesReservedAnnoPodNamespace,
		kubernetesReservedAnnoPodAttempt,
	}
}

// parseContainerID parses the container ID string into "uuid" + "appname".
// containerID will be "${uuid}:${attempt}:${containerName}".
// So the result will be "${uuid}", and "${attempt}-${containerName}".
func parseContainerID(containerID string) (uuid, appName string, err error) {
	values := strings.SplitN(containerID, ":", 2)
	if len(values) != 2 {
		return "", "", fmt.Errorf("invalid container ID %q", containerID)
	}
	return values[0], values[1], nil
}

func buildContainerID(uuid, appName string) (containerID string) {
	return fmt.Sprintf("%s:%s", uuid, appName)
}

// parseAppName parses the app name string into "attempt" + "container name".
// appName will be "${attempt}:${containerName}".
func parseAppName(appName string) (attempt uint32, containerName string, err error) {
	values := strings.SplitN(appName, "-", 2)
	if len(values) != 2 {
		return 0, "", fmt.Errorf("invalid appName %q", appName)
	}

	a, err := strconv.ParseUint(values[0], 10, 32)
	if err != nil {
		return 0, "", fmt.Errorf("invalid appName %q", appName)
	}

	return uint32(a), values[1], nil
}

func buildAppName(attempt uint32, containerName string) string {
	return fmt.Sprintf("%d-%s", attempt, containerName)
}

func toContainerStatus(uuid string, app *rkt.App) (*runtimeApi.ContainerStatus, error) {
	var status runtimeApi.ContainerStatus

	id := buildContainerID(uuid, app.Name)
	status.Id = &id

	attempt, containerName, err := parseAppName(app.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse app name: %v", err)
	}

	status.Metadata = &runtimeApi.ContainerMetadata{
		Name:    &containerName,
		Attempt: &attempt,
	}

	state := runtimeApi.ContainerState_UNKNOWN
	switch app.State {
	case rkt.AppStateUnknown:
		state = runtimeApi.ContainerState_UNKNOWN
	case rkt.AppStateCreated:
		state = runtimeApi.ContainerState_CREATED
	case rkt.AppStateRunning:
		state = runtimeApi.ContainerState_RUNNING
	case rkt.AppStateExited:
		state = runtimeApi.ContainerState_EXITED
	default:
		state = runtimeApi.ContainerState_UNKNOWN
	}

	status.State = &state
	status.CreatedAt = app.CreatedAt
	status.StartedAt = app.StartedAt
	status.FinishedAt = app.FinishedAt
	status.ExitCode = app.ExitCode
	status.ImageRef = &app.ImageID
	status.Image = &runtimeApi.ImageSpec{Image: getImageName(app.CRIAnnotations)}

	status.Labels = getKubernetesLabels(app.CRILabels)
	status.Annotations = getKubernetesAnnotations(app.CRIAnnotations)

	// TODO: Make sure mount name is unique.
	for _, mnt := range app.Mounts {
		status.Mounts = append(status.Mounts, &runtimeApi.Mount{
			Name:          &mnt.Name,
			ContainerPath: &mnt.ContainerPath,
			HostPath:      &mnt.HostPath,
			Readonly:      &mnt.ReadOnly,
			// TODO: Selinux relabeling.
		})
	}

	return &status, nil
}

func getKubernetesLabels(labels map[string]string) map[string]string {
	return labels
}

func getKubernetesAnnotations(annotations map[string]string) map[string]string {
	if len(annotations) == 0 {
		return nil
	}

	ret := annotations
	for _, key := range kubernetesReservedAnnoKeys {
		delete(ret, key)
	}
	return ret
}

func getImageName(annotations map[string]string) *string {
	name := annotations[kubernetesReservedAnnoImageNameKey]
	return &name
}

func generateAppAddCommand(req *runtimeApi.CreateContainerRequest, imageID string) []string {
	// Generate labels and annotations.
	var labels, annotations []string
	for k, v := range req.Config.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range req.Config.Annotations {
		annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
	}
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoImageNameKey, *req.Config.Image.Image))

	// Generate app name.
	appName := buildAppName(*req.Config.Metadata.Attempt, *req.Config.Metadata.Name)

	// Generate the command and arguments for 'rkt app add'.
	cmd := []string{"app", "add", *req.PodSandboxId, imageID, "--name=" + appName}
	for _, anno := range annotations {
		cmd = append(cmd, "--annotation="+anno)
	}
	for _, label := range labels {
		cmd = append(cmd, "--label="+label)
	}
	return cmd
}

func generateAppSandboxCommand(req *runtimeApi.RunPodSandboxRequest, uuidfile string) []string {
	cmd := []string{"app", "sandbox", "--uuid-file-save=" + uuidfile}

	// TODO(yifan): Namespace options.

	// Generate annotations.
	var labels, annotations []string
	for k, v := range req.Config.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range req.Config.Annotations {
		annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
	}

	// Reserved annotations.
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoPodUid, *req.Config.Metadata.Uid))
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoPodName, *req.Config.Metadata.Name))
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoPodNamespace, *req.Config.Metadata.Namespace))
	annotations = append(annotations, fmt.Sprintf("%s=%d", kubernetesReservedAnnoPodAttempt, *req.Config.Metadata.Attempt))

	for _, anno := range annotations {
		cmd = append(cmd, "--annotation="+anno)
	}
	for _, label := range labels {
		cmd = append(cmd, "--label="+label)
	}
	return cmd
}

func getKubernetesMetadata(annotations map[string]string) (*runtimeApi.PodSandboxMetadata, error) {
	podUid := annotations[kubernetesReservedAnnoPodUid]
	podName := annotations[kubernetesReservedAnnoPodName]
	podNamespace := annotations[kubernetesReservedAnnoPodNamespace]
	podAttemptStr := annotations[kubernetesReservedAnnoPodAttempt]

	attempt, err := strconv.ParseUint(podAttemptStr, 10, 32)
	if err != nil {
		return nil, fmt.Errorf("error parsing attempt count %q: %v", podAttemptStr, err)
	}
	podAttempt := uint32(attempt)

	return &runtimeApi.PodSandboxMetadata{
		Uid:       &podUid,
		Name:      &podName,
		Namespace: &podNamespace,
		Attempt:   &podAttempt,
	}, nil
}

func toPodSandboxStatus(pod *rkt.Pod) (*runtimeApi.PodSandboxStatus, error) {
	metadata, err := getKubernetesMetadata(pod.CRIAnnotations)
	if err != nil {
		return nil, err
	}

	var startedAt int64
	if pod.StartedAt != nil {
		startedAt = *pod.StartedAt
	}

	state := runtimeApi.PodSandBoxState_NOTREADY
	if pod.State == "running" {
		state = runtimeApi.PodSandBoxState_READY
	}

	ip := getIP(pod.Networks)

	return &runtimeApi.PodSandboxStatus{
		Id:          &pod.UUID,
		Metadata:    metadata,
		State:       &state,
		CreatedAt:   &startedAt,
		Network:     &runtimeApi.PodSandboxNetworkStatus{Ip: &ip},
		Linux:       nil, // TODO
		Labels:      getKubernetesLabels(pod.CRILabels),
		Annotations: getKubernetesAnnotations(pod.CRIAnnotations),
	}, nil
}

// getIP returns the ip of the pod.
// The ip of a network named rkt.kubernetes.io will be preferred, followed by
// default, followed by the first one
// The input might look something like 'default:ip4=172.16.28.27,foo:ip4=x.y.z.a'
func getIP(networks []netinfo.NetInfo) string {
	var foundIP string
	for _, network := range networks {

		// Always prefer this network if available.
		// We're done if we find it
		if network.NetName == "rkt.kubernetes.io" {
			return network.IP.To4().String()
		}

		// Even if we already have a previous ip,
		// prefer default over it.
		// If it was rkt.kubernetes.io, we already returned,
		// so it must have been an arbitrary one.
		if network.NetName == "default" {
			foundIP = network.IP.To4().String()
		}

		// If nothing else has matched, we can use this one,
		// but keep going to see if we find 'default' or
		// 'rkt.kubernetes.io'.
		if foundIP == "" {
			foundIP = network.IP.To4().String()
		}
	}
	return foundIP
}
