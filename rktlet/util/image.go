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

package util

import (
	"fmt"
	"regexp"
	"strings"

	dockerref "github.com/docker/distribution/reference"
	"k8s.io/kubernetes/pkg/util/parsers"
)

const dockerPrefix string = "docker://"

var (
	HashRegexp = regexp.MustCompile(`^(sha1|sha2|sha256|sha512)-.*$`)
)

// TODO(euank): this is taken from kubelet/image/image_manager.go.
// "A little copying is better than a little dependency." -- https://go-proverbs.github.io/
// This should not exist here, the kubelet should break out image and tag in
// the ImageStatusRequest and then we can leave it to parse.
// applyDefaultImageTag parses a docker image string, if it doesn't contain any tag or digest,
// a default tag will be applied.
func ApplyDefaultImageTag(image string) (string, error) {
	named, err := dockerref.ParseNormalizedNamed(image)
	if err != nil {
		return "", fmt.Errorf("couldn't parse image reference %q: %v", image, err)
	}
	_, isTagged := named.(dockerref.Tagged)
	_, isDigested := named.(dockerref.Digested)
	if !isTagged && !isDigested {
		named, err := dockerref.WithTag(named, parsers.DefaultImageTag)
		if err != nil {
			return "", fmt.Errorf("failed to apply default image tag %q: %v", image, err)
		}
		image = named.String()
	}
	return image, nil
}

func GetCanonicalImageName(imageName string) (string, error) {
	var err error
	imgName := strings.TrimPrefix(imageName, dockerPrefix)
	if HashRegexp.FindString(imgName) != "" {
		return imgName, nil
	}

	canonicalImageName, err := ApplyDefaultImageTag(imgName)
	if err != nil {
		return "", fmt.Errorf("unable to apply default tag for img %q, %v", imageName, err)
	}

	imageID := canonicalImageName
	if !strings.HasPrefix(canonicalImageName, dockerPrefix) {
		imageID = dockerPrefix + canonicalImageName
	}

	return imageID, nil
}

func ExistInSlice(inSlice []string, key string) bool {
	for _, v := range inSlice {
		if v == key {
			return true
		}
	}
	return false
}
