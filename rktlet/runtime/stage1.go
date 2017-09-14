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
	"context"
	"fmt"

	"github.com/golang/glog"
)

func (r *RktRuntime) fetchStage1Image(ctx context.Context) error {
	if r.stage1Name == "" {
		return nil
	}

	_, err := r.getImageHash(ctx, r.stage1Name)
	if err == nil {
		return nil
	}

	glog.Infof("downloading %q stage1 image, this may take some time", r.stage1Name)
	output, err := r.RunCommand("image", "fetch", "--pull-policy=update", "--full=true", r.stage1Name)
	if err != nil {
		return fmt.Errorf("unable to fetch image %q: %v", r.stage1Name, err)
	}
	if len(output) < 1 {
		return fmt.Errorf("malformed fetch image response for %q; must include image id: %v", r.stage1Name, output)
	}
	glog.Infof("finished downloading stage1 image")

	return err
}
