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

import "regexp"

// pod "379ae074-f1ca-4bdc-8493-e7278b00009f" is already stopped
var alreadyStoppedRegex = regexp.MustCompile(`^pod "[^"]+" is already stopped$`)

// RktStopIsNotExistError determines if an error resulting from running `rkt
// stop` or `rkt app stop` is the result of the pod already being stopped
func RktStopIsAlreadyStoppedError(err error) bool {
	if err == nil {
		return false
	}
	return alreadyStoppedRegex.MatchString(err.Error())
}

// stop: cannot get pod: no matches found for "37edaae0-f048-4db5-b3fb-c0de3aa8e9d8"
var isNotExistStopError = regexp.MustCompile(`^stop: cannot get pod: no matches found for "[^"]+"$`)

func RktStopIsNotExistError(err error) bool {
	if err == nil {
		return false
	}
	return isNotExistStopError.MatchString(err.Error())
}
