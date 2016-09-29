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

package image

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"

	context "golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// TODO(tmrts): Move these errors to the container API for code re-use.
var (
	ErrImageNotFound = errors.New("rkt: image not found")
)

// var _ kubeletApi.ImageManagerService = (*ImageStore)(nil)

// ImageStore supports CRUD operations for images.
type ImageStore struct {
	cli.CLI
	requestTimeout time.Duration
}

// TODO(tmrts): fill the image store configuration fields.
type ImageStoreConfig struct {
	CLI            cli.CLI
	RequestTimeout time.Duration
}

// NewImageStore creates an image storage that allows CRUD operations for images.
func NewImageStore(cfg ImageStoreConfig) runtime.ImageServiceServer {
	return &ImageStore{cfg.CLI, cfg.RequestTimeout}
}

// Remove removes the image from the image store.
func (s *ImageStore) RemoveImage(ctx context.Context, req *runtime.RemoveImageRequest) (*runtime.RemoveImageResponse, error) {
	img, err := s.ImageStatus(ctx, &runtime.ImageStatusRequest{Image: req.Image})
	if err != nil {
		return nil, err
	}

	if _, err := s.RunCommand("image", "rm", *img.Image.Id); err != nil {
		return nil, fmt.Errorf("failed to remove the image: %v", err)
	}

	return &runtime.RemoveImageResponse{}, nil
}

// ImageStatus returns the status of the image.
// TODO(euank): rkt should support listing a single image so this is more
// efficient
func (s *ImageStore) ImageStatus(ctx context.Context, req *runtime.ImageStatusRequest) (*runtime.ImageStatusResponse, error) {
	images, err := s.ListImages(ctx, &runtime.ListImagesRequest{})
	if err != nil {
		return nil, err
	}

	for _, img := range images.Images {
		for _, name := range img.RepoTags {
			if name == *req.Image.Image {
				return &runtime.ImageStatusResponse{Image: img}, nil
			}
		}
	}

	return nil, fmt.Errorf("couldn't to find the image %v", *req.Image.Image)
}

// ListImages lists images in the store
func (s *ImageStore) ListImages(ctx context.Context, req *runtime.ListImagesRequest) (*runtime.ListImagesResponse, error) {
	list, err := s.RunCommand("image", "list",
		"--full",
		"--no-legend",
		"--fields=id,name,size",
		"--sort=importtime",
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't list images: %v", err)
	}

	images := make([]*runtime.Image, 0, len(list))
	for _, image := range list {
		tokens := strings.Fields(image)
		if len(tokens) < 2 {
			glog.Errorf("malformed line in image list: %v", image)
			continue
		}
		id, name := tokens[0], tokens[1]
		size, err := strconv.ParseUint(tokens[2], 10, 0)
		if err != nil {
			return nil, fmt.Errorf("invalid image size format: %v", err)
		}

		image := &runtime.Image{
			Id: &id,
			// TODO(yifan): Why not just call it name.
			RepoTags: []string{name},
			// TODO(yifan): Rename this field to something more generic?
			RepoDigests: []string{id},
			Size_:       &size,
		}

		if passFilter(image, req.Filter) {
			images = append(images, image)
		}
	}

	return &runtime.ListImagesResponse{Images: images}, nil
}

// PullImage pulls an image into the store
func (s *ImageStore) PullImage(ctx context.Context, req *runtime.PullImageRequest) (*runtime.PullImageResponse, error) {
	// TODO auth
	output, err := s.RunCommand("image", "fetch", "--no-store=true", "--insecure-options=image,ondisk", "--full=true", "docker://"+*req.Image.Image)

	if err != nil {
		return nil, fmt.Errorf("unable to fetch image: %v", err)
	}
	if len(output) < 1 {
		return nil, fmt.Errorf("malformed fetch image response; must include image id: %v", output)
	}

	return &runtime.PullImageResponse{}, nil
}

// passFilter returns whether the target image satisfies the filter.
func passFilter(image *runtime.Image, filter *runtime.ImageFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Image == nil {
		return true
	}

	imageName := filter.Image.GetImage()
	for _, name := range image.RepoTags {
		if imageName == name {
			return true
		}
	}
	return false
}
