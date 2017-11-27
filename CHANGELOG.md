## 0.1.0

This is the first release of rktlet.
It implements basic support for the CRI including fetching images, running pods, CNI networking, logging and exec.

For this release, rktlet passes 129/145 kubernetes e2e conformance tests on Kubernetes version v1.9.0-alpha.2.328+d07bc1485cddc9.

Some features like attach and port forwarding don't work yet, we'll be focusing on those over the next releases.
