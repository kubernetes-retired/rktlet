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
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/rktlet/rktlet/cli"
	"github.com/kubernetes-incubator/rktlet/rktlet/util"

	appcschema "github.com/appc/spec/schema"
	rktlib "github.com/rkt/rkt/api/v1"
	context "golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/kubelet/apis/cri/v1alpha1/runtime"
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
	if img.Image == nil {
		return nil, fmt.Errorf("Image does not exist")
	}

	if output, err := s.RunCommand("image", "rm", img.Image.Id); err != nil {
		return nil, fmt.Errorf("failed to remove the image, output: %s\nerr: %v", output, err)
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

	reqImg := req.Image.Image
	// TODO this should be done in kubelet (see comment on ApplyDefaultImageTag)
	reqImg, err = util.ApplyDefaultImageTag(reqImg)
	if err != nil {
		return nil, err
	}

	for _, img := range images.Images {
		for _, name := range img.RepoTags {
			if name == reqImg {
				return &runtime.ImageStatusResponse{Image: img}, nil
			}
		}
	}

	// api expected response for "Image does not exist"
	return &runtime.ImageStatusResponse{}, nil
}

// ListImages lists images in the store
func (s *ImageStore) ListImages(ctx context.Context, req *runtime.ListImagesRequest) (*runtime.ListImagesResponse, error) {
	list, err := s.RunCommand("image", "list",
		"--full",
		"--format=json",
		"--sort=importtime",
	)
	if err != nil {
		return nil, fmt.Errorf("couldn't list images: %v", err)
	}

	listEntries := []rktlib.ImageListEntry{}

	err = json.Unmarshal([]byte(list[0]), &listEntries)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal images into expected format: %v", err)
	}

	images := make([]*runtime.Image, 0, len(list))
	for i, _ := range listEntries {
		img := listEntries[i]

		var realName, user string
		manifest, err := s.getImageManifest(img.ID)
		if err != nil {
			glog.Warningf("unable to get image %q manifest: %v", img.ID, err)
			realName = img.Name
			user = ""
		} else {
			realName = s.getImageRealName(manifest, img.Name)
			user = s.getImageUser(manifest)
		}

		sz := uint64(img.Size)
		image := &runtime.Image{
			Id:          img.ID,
			RepoTags:    []string{realName},
			RepoDigests: []string{img.ID},
			Size_:       sz,
		}
		if uid, err := strconv.ParseInt(user, 10, 64); err != nil {
			image.Uid = &runtime.Int64Value{uid}
		} else {
			image.Username = user
		}

		if passFilter(image, req.Filter) {
			images = append(images, image)
		}
	}

	return &runtime.ListImagesResponse{Images: images}, nil
}

// ImageFSInfo returns information of the filesystem that is used to store images.
func (s *ImageStore) ImageFsInfo(ctx context.Context, req *runtime.ImageFsInfoRequest) (*runtime.ImageFsInfoResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *ImageStore) getImageManifest(id string) (*appcschema.ImageManifest, error) {
	imgManifest, err := s.RunCommand("image", "cat-manifest", id)
	if err != nil {
		return nil, err
	}
	var manifest appcschema.ImageManifest

	err = json.Unmarshal([]byte(strings.Join(imgManifest, "")), &manifest)
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (s *ImageStore) getImageRealName(manifest *appcschema.ImageManifest, default_ string) string {
	originalName, ok := manifest.GetAnnotation("appc.io/docker/originalname")
	if !ok {
		glog.V(3).Infof(
			"image %q does not have originalname annotation, reverting to default %q",
			manifest.Name.String(),
			default_,
		)
		return default_
	}
	// Normalize in case someone typed `rkt fetch docker://image-without-tag`
	// because our kubelet version will have that latest tag.
	realName, err := util.ApplyDefaultImageTag(originalName)
	if err != nil {
		glog.Warningf("error adding default tag to image %v, %v", originalName, err)
		return originalName
	}
	return realName
}

func (s *ImageStore) getImageUser(manifest *appcschema.ImageManifest) string {
	if manifest.App != nil {
		return manifest.App.User
	}
	return ""
}

// PullImage pulls an image into the store
func (s *ImageStore) PullImage(ctx context.Context, req *runtime.PullImageRequest) (*runtime.PullImageResponse, error) {

	canonicalImageName, err := util.ApplyDefaultImageTag(req.Image.Image)
	if err != nil {
		return nil, fmt.Errorf("unable to default tag for img %q, %v", req.Image.Image, err)
	}

	// TODO auth
	output, err := s.RunCommand("image", "fetch", "--pull-policy=update", "--full=true", "docker://"+canonicalImageName)
	if err != nil {
		return nil, fmt.Errorf("unable to fetch image %q\noutput: %s\nerr: %v", canonicalImageName, output, err)
	}
	if len(output) < 1 {
		return nil, fmt.Errorf("malformed fetch image response for %q; must include image id: %v", canonicalImageName, output)
	}
	imageId := output[len(output)-1]

	return &runtime.PullImageResponse{
		ImageRef: imageId,
	}, nil
}

// passFilter returns whether the target image satisfies the filter.
func passFilter(image *runtime.Image, filter *runtime.ImageFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Image == nil {
		return true
	}

	imageName := filter.Image.Image
	for _, name := range image.RepoTags {
		if imageName == name {
			return true
		}
	}
	return false
}
