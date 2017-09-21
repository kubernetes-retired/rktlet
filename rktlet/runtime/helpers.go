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
	"os"
	"strconv"
	"strings"

	"github.com/golang/glog"
	"github.com/pborman/uuid"
	rkt "github.com/rkt/rkt/api/v1"
	"github.com/rkt/rkt/networking/netinfo"
	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api/v1"
	runtimeApi "k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
)

const (
	// Exists per app.
	kubernetesReservedAnnoImageNameKey = "k8s.io/reserved/image-name"

	// Exists per pod.
	kubernetesReservedAnnoPodUid       = "k8s.io/reserved/pod-uid"
	kubernetesReservedAnnoPodName      = "k8s.io/reserved/pod-name"
	kubernetesReservedAnnoPodNamespace = "k8s.io/reserved/pod-namespace"
	kubernetesReservedAnnoPodAttempt   = "k8s.io/reserved/pod-attempt"

	// TODO(euank): This has significant security concerns as a stage1 image is
	// effectively root.
	// Furthermore, this (using an annotation) is a hack to pass an extra
	// non-portable argument in. It should not be relied on to be stable.
	// In the future, this might be subsumed by a first-class api object, or by a
	// kitchen-sink params object (#17064).
	// See discussion in #23944
	// Also, do we want more granularity than path-at-the-kubelet-level and
	// image/name-at-the-pod-level?
	k8sRktStage1NameAnno = "rkt.alpha.kubernetes.io/stage1-name-override"
)

// List of reserved keys in the annotations.
var kubernetesReservedAnnoKeys = []string{
	kubernetesReservedAnnoImageNameKey,
	kubernetesReservedAnnoPodUid,
	kubernetesReservedAnnoPodName,
	kubernetesReservedAnnoPodNamespace,
	kubernetesReservedAnnoPodAttempt,
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
	status.Id = id

	attempt, containerName, err := parseAppName(app.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to parse app name: %v", err)
	}

	status.Metadata = &runtimeApi.ContainerMetadata{
		Name:    containerName,
		Attempt: attempt,
	}

	state := runtimeApi.ContainerState_CONTAINER_UNKNOWN
	switch app.State {
	case rkt.AppStateUnknown:
		state = runtimeApi.ContainerState_CONTAINER_UNKNOWN
	case rkt.AppStateCreated:
		state = runtimeApi.ContainerState_CONTAINER_CREATED
	case rkt.AppStateRunning:
		state = runtimeApi.ContainerState_CONTAINER_RUNNING
	case rkt.AppStateExited:
		state = runtimeApi.ContainerState_CONTAINER_EXITED
	default:
		state = runtimeApi.ContainerState_CONTAINER_UNKNOWN
	}

	status.State = state
	status.CreatedAt = nilToZero64(app.CreatedAt)
	status.StartedAt = nilToZero64(app.StartedAt)
	status.FinishedAt = nilToZero64(app.FinishedAt)
	status.ExitCode = nilToZero32(app.ExitCode)
	status.ImageRef = app.ImageID
	status.Image = &runtimeApi.ImageSpec{Image: getImageName(app.UserAnnotations)}

	status.Labels = getKubernetesLabels(app.UserLabels)
	status.Annotations = getKubernetesAnnotations(app.UserAnnotations)

	for _, mnt := range app.Mounts {
		status.Mounts = append(status.Mounts, &runtimeApi.Mount{
			ContainerPath: mnt.ContainerPath,
			HostPath:      mnt.HostPath,
			Readonly:      mnt.ReadOnly,
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

func getImageName(annotations map[string]string) string {
	name := annotations[kubernetesReservedAnnoImageNameKey]
	return name
}

func generateSeccompArg(annotations map[string]string, containerName string) (string, error) {
	// by default kubernetes doesn't enable seccomp
	defaultSeccomp := "--seccomp=mode=retain,@appc.io/all"

	profile, ok := annotations[v1.SeccompContainerAnnotationKeyPrefix+containerName]
	if !ok {
		profile, ok = annotations[v1.SeccompPodAnnotationKey]
		if !ok {
			return defaultSeccomp, nil
		}
	}

	if profile == "unconfined" {
		return defaultSeccomp, nil
	}

	if profile == "docker/default" {
		// use rkt default's, which matches docker's
		return "", nil
	}

	// TODO(iaguis): handle custom profiles

	return "", fmt.Errorf("seccomp profile %q not supported", profile)
}

func generateAppAddCommand(req *runtimeApi.CreateContainerRequest, imageID string) ([]string, error) {
	config := req.Config

	// Generate labels and annotations.
	var labels, annotations []string
	for k, v := range config.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range config.Annotations {
		annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
	}
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoImageNameKey, config.Image.Image))

	// Generate app name.
	appName := buildAppName(config.Metadata.Attempt, config.Metadata.Name)

	// TODO(yifan): Split the function into sub-functions.
	// Generate the command and arguments for 'rkt app add'.
	cmd := []string{"app", "add", req.PodSandboxId, imageID}

	// Add app name
	cmd = append(cmd, "--name="+appName)

	// Add annotations and labels.
	for _, anno := range annotations {
		cmd = append(cmd, "--user-annotation="+anno)
	}
	for _, label := range labels {
		cmd = append(cmd, "--user-label="+label)
	}

	// Add environments
	for _, env := range config.Envs {
		cmd = append(cmd, fmt.Sprintf("--environment=%s=%s", env.Key, env.Value))
	}

	// Add Linux options. (resources, caps, uid, gid).
	if linux := config.GetLinux(); linux != nil {
		// Add resources.
		if resources := linux.Resources; resources != nil {
			if resources.CpuShares > 0 {
				cmd = append(cmd, fmt.Sprintf("--cpu-shares=%d", resources.CpuShares))
			}
			var cpuMilliCores int64
			cpuMilliCores = cpuQuotaToMilliCores(resources.CpuQuota, resources.CpuPeriod)
			if cpuMilliCores > 0 {
				cmd = append(cmd, fmt.Sprintf("--cpu=%dm", cpuMilliCores))
			}

			if resources.MemoryLimitInBytes != 0 {
				cmd = append(cmd, fmt.Sprintf("--memory=%d", resources.MemoryLimitInBytes))
			}
			if resources.OomScoreAdj != 0 {
				cmd = append(cmd, fmt.Sprintf("--oom-score-adj=%d", resources.OomScoreAdj))
			}
		}

		// Add capabilities.
		// TODO(yifan): Update the implementation of the capability
		// once upstream has adopted the whitelist based interface.
		// See https://github.com/kubernetes/kubernetes/pull/33614.
		var caplist []string
		if secContext := linux.GetSecurityContext(); secContext != nil {
			if secContext.Privileged {
				cmd = append(cmd, "--seccomp=mode=retain,@appc.io/all")
				caplist = getAllCapabilites()
				// TODO: device cgroup should be made permissive
				// TODO: host's /dev's devices should all be visible in the container
			} else {
				seccompArg, err := generateSeccompArg(config.Annotations, config.Metadata.Name)
				if err != nil {
					return nil, err
				}
				cmd = append(cmd, seccompArg)

				if secContext.Capabilities != nil {
					caplist, err = tweakCapabilities(defaultCapabilities, secContext.Capabilities.AddCapabilities, secContext.Capabilities.DropCapabilities)
					if err != nil {
						return nil, err
					}
				}
			}
			if len(caplist) > 0 {
				cmd = append(cmd, "--caps-retain="+strings.Join(caplist, ","))
			}

			if secContext.RunAsUser != nil && secContext.RunAsUsername != "" {
				return nil, fmt.Errorf("invalid request; both username and user fields of SecurityContext set")
			}
			// Add uid, addtional gids.
			if secContext.RunAsUser != nil {
				cmd = append(cmd, fmt.Sprintf("--user=%d", (*secContext.RunAsUser).Value))
			}
			if secContext.RunAsUsername != "" {
				cmd = append(cmd, fmt.Sprintf("--user=%s", secContext.RunAsUsername))
			}

			if len(secContext.SupplementalGroups) > 0 {
				var gids []string
				for _, gid := range secContext.SupplementalGroups {
					gids = append(gids, fmt.Sprintf("%d", gid))
				}
				cmd = append(cmd, "--supplementary-gids="+strings.Join(gids, ","))
			}

			// Add ReadOnlyRootFs.
			if secContext.ReadonlyRootfs {
				cmd = append(cmd, "--readonly-rootfs=true")
			}
		}

		// TODO(yifan): Figure out selinux,
		// https://github.com/kubernetes/kubernetes/issues/33139

	}

	// Add working dir
	if config.WorkingDir != "" {
		cmd = append(cmd, "--working-dir="+config.WorkingDir)
	}

	for _, mnt := range config.GetMounts() {
		if mnt == nil {
			glog.Warningf("unexpected nil mount: %v, %+v", mnt, config)
			continue
		}
		volumeName := uuid.NewUUID()
		cmd = append(cmd, fmt.Sprintf("--mnt-volume=name=%s,kind=host,source=%s,target=%s,readOnly=%t", volumeName, mnt.HostPath, mnt.ContainerPath, mnt.Readonly))
	}

	// Add app commands and args.
	var args []string
	if len(config.Command) > 0 {
		cmd = append(cmd, "--exec="+config.Command[0])
		args = append(args, config.Command[1:]...)
	}
	if len(config.Args) > 0 {
		args = append(args, config.Args...)
	}
	if len(args) > 0 {
		cmd = append(cmd, "--")
		cmd = append(cmd, args...)
		cmd = append(cmd, "---")
	}

	return cmd, nil
}

func generateAppSandboxCommand(req *runtimeApi.RunPodSandboxRequest, uuidfile, stage1Name string) []string {
	cmd := []string{"app", "sandbox", "--uuid-file-save=" + uuidfile}

	// annotation takes preference over configuration
	if val, ok := req.Config.Annotations[k8sRktStage1NameAnno]; ok {
		stage1Name = val
	}

	if stage1Name != "" {
		cmd = append(cmd, "--stage1-name="+stage1Name)
	}

	if req.Config.Hostname != "" {
		cmd = append(cmd, "--hostname="+req.Config.Hostname)
	}

	// Add DNS options.
	if config := req.Config.DnsConfig; config != nil {
		for _, server := range config.Servers {
			cmd = append(cmd, "--dns="+server)
		}
		for _, search := range config.Searches {
			cmd = append(cmd, "--dns-search="+search)
		}
		for _, opt := range config.Options {
			cmd = append(cmd, "--dns-opt="+opt)
		}
	}

	if sc := req.GetConfig().GetLinux().GetSecurityContext(); sc != nil && sc.Privileged {
		// TODO: the 'paths' setting is applied to all applications even though only
		// a subset of them may request to be privileged, however there is no way
		// to modify paths on a per-app basis currently, so this is the best we can
		// do.
		cmd = append(cmd, "--insecure-options=all-run")
	}

	// Add port mappings only if it's not hostnetwork.
	if !hasHostNetwork(req.GetConfig()) {
		for _, portMapping := range req.Config.PortMappings {
			if portMapping.HostPort == 0 || portMapping.ContainerPort == 0 {
				// If no host port is specified, then ignore.
				// TODO(yifan): Do this check in kubelet.
				continue
			}
			portArg := generatePortArgs(portMapping)
			cmd = append(cmd, portArg)
		}
	}

	// Add hostnetwork
	if hasHostNetwork(req.GetConfig()) {
		cmd = append(cmd, "--net=host", "--hosts-entry=host")
		if hn, err := os.Hostname(); err == nil {
			cmd = append(cmd, fmt.Sprintf("--hostname=%s", hn))
		}
	}

	// Generate annotations.
	var labels, annotations []string
	for k, v := range req.Config.Labels {
		labels = append(labels, fmt.Sprintf("%s=%s", k, v))
	}
	for k, v := range req.Config.Annotations {
		annotations = append(annotations, fmt.Sprintf("%s=%s", k, v))
	}

	// Reserved annotations.
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoPodUid, req.Config.GetMetadata().Uid))
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoPodName, req.Config.Metadata.Name))
	annotations = append(annotations, fmt.Sprintf("%s=%s", kubernetesReservedAnnoPodNamespace, req.Config.Metadata.Namespace))
	annotations = append(annotations, fmt.Sprintf("%s=%d", kubernetesReservedAnnoPodAttempt, req.GetConfig().GetMetadata().Attempt))

	for _, anno := range annotations {
		cmd = append(cmd, "--user-annotation="+anno)
	}
	for _, label := range labels {
		cmd = append(cmd, "--user-label="+label)
	}

	return cmd
}

// isKubernetesPod determines if the pod is actually owned by Kubernetes.
// It checks for critical annotations.
func isKubernetesPod(pod *rkt.Pod) bool {
	_, ok := pod.UserAnnotations[kubernetesReservedAnnoPodUid]
	return ok
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
		Uid:       podUid,
		Name:      podName,
		Namespace: podNamespace,
		Attempt:   podAttempt,
	}, nil
}

func toPodSandboxStatus(pod *rkt.Pod) (*runtimeApi.PodSandboxStatus, error) {
	metadata, err := getKubernetesMetadata(pod.UserAnnotations)
	if err != nil {
		return nil, err
	}

	var createdAt int64
	if pod.CreatedAt != nil {
		createdAt = *pod.CreatedAt
	}

	state := runtimeApi.PodSandboxState_SANDBOX_NOTREADY
	if pod.State == "running" {
		state = runtimeApi.PodSandboxState_SANDBOX_READY
	}

	ip := getIP(pod.Networks) // TODO: no network case

	return &runtimeApi.PodSandboxStatus{
		Id:          pod.UUID,
		Metadata:    metadata,
		State:       state,
		CreatedAt:   createdAt,
		Network:     &runtimeApi.PodSandboxNetworkStatus{Ip: ip},
		Linux:       nil, // TODO
		Labels:      getKubernetesLabels(pod.UserLabels),
		Annotations: getKubernetesAnnotations(pod.UserAnnotations),
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

// passFilter returns whether the target container satisfies the filter.
func passFilter(container *runtimeApi.Container, filter *runtimeApi.ContainerFilter) bool {
	if filter == nil {
		return true
	}
	if filter.Id != "" && filter.Id != container.Id {
		return false
	}
	if filter.GetState() != nil && filter.GetState().State != container.State {
		return false
	}
	if filter.PodSandboxId != "" && filter.PodSandboxId != container.PodSandboxId {
		return false
	}
	for key, value := range filter.LabelSelector {
		v, ok := container.Labels[key]
		if !ok || value != v {
			return false
		}
	}
	return true
}

func cpuSharesToMilliCores(cpushare int64) int64 {
	return cpushare * 1000 / 1024
}

func cpuQuotaToMilliCores(cpuQuota, cpuPeriod int64) int64 {
	if cpuQuota == 0 || cpuPeriod == 0 {
		return 0
	}
	return cpuQuota * 1000 / cpuPeriod
}

// generatePortArgs returns the `--port` argument derived from the port mapping.
func generatePortArgs(port *runtimeApi.PortMapping) string {
	protocol := strings.ToLower(port.Protocol.String())
	containerPort := port.ContainerPort
	hostPort := port.HostPort
	hostIP := port.HostIp
	if hostIP == "" {
		hostIP = "0.0.0.0"
	}
	// The name is in the format of "protocol-containerPort-hostPort",
	// e.g. "tcp-80-8080", which satisfies the ACName format.
	name := fmt.Sprintf("%s-%d-%d", protocol, containerPort, hostPort)

	return fmt.Sprintf("--port=%s:%s:%d:%s:%d", name, protocol, containerPort, hostIP, hostPort)
}

func hasHostNetwork(req *runtimeApi.PodSandboxConfig) bool {
	if nsOpts := req.GetLinux().GetSecurityContext().GetNamespaceOptions(); nsOpts != nil {
		return nsOpts.HostNetwork
	}

	return false
}

func (r *RktRuntime) getImageHash(ctx context.Context, imageName string) (string, error) {
	resp, err := r.imageStore.ImageStatus(ctx, &runtimeApi.ImageStatusRequest{
		Image: &runtimeApi.ImageSpec{
			Image: imageName,
		},
	})
	if err != nil {
		return "", fmt.Errorf("unable to get status for image %q: %v", imageName, err)
	}
	if resp.GetImage() == nil {
		return "", fmt.Errorf("could not find image %q", imageName)
	}

	return resp.GetImage().Id, nil
}

func podSandboxStatusMatchesFilter(sbx *runtimeApi.PodSandboxStatus, filter *runtimeApi.PodSandboxFilter) bool {
	if filter == nil {
		return true
	}
	if filter.Id != "" && filter.Id != sbx.Id {
		return false
	}

	if filter.State != nil && filter.GetState().State != sbx.State {
		return false
	}

	for key, val := range filter.LabelSelector {
		sbxLabel, exists := sbx.Labels[key]
		if !exists {
			return false
		}
		if sbxLabel != val {
			return false
		}
	}

	return true
}

func nilToZero64(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}
func nilToZero32(i *int32) int32 {
	if i == nil {
		return 0
	}
	return *i
}
