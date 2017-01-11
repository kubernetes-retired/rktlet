#!/bin/bash

# Copyright 2015 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Update all Jenkins jobs in a folder specified in $1. It can be the union of
# multiple folders separated with a colon, like with the PATH variable.

config_dir="${1:-"jenkins/job-configs/kubernetes-jenkins"}"

# Run the container if it isn't present.
if ! docker inspect job-builder-local &> /dev/null; then
  # jenkins_jobs.ini contains administrative credentials for Jenkins.
  # Store it in the workspace of the Jenkins job that calls this script.
  if [[ -e jenkins_jobs.ini ]]; then
    docker run -idt \
      --net host \
      --name job-builder-local \
      --volume "$(pwd):/test-infra/jenkins" \
      --restart always \
      gcr.io/google_containers/kubekins-job-builder:5
    docker cp jenkins_jobs.ini job-builder-local:/etc/jenkins_jobs
  else
    echo "jenkins_jobs.ini not found" >&2
    exit 1
  fi
fi

if ! docker inspect rktnetes-jenkins-proxy &> /dev/null; then
  docker run --name="rktnetes-jenkins-proxy" \
    -d --net=host \
    -e PROXY_TO=127.0.0.1:8080 \
    -e EXTRA_HEADER='X-Forwarded-User: rktnetes-deploy-bot' \
    quay.io/euank/nginx-proxy-header
  sleep 1
fi

docker exec job-builder-local jenkins-jobs update ${config_dir}
