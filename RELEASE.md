# Release Process

The Rktlet Project is released on an as-needed basis. The process is as follows:

1. An issue is proposing a new release with a changelog since the last release
1. All [OWNERS](OWNERS) must LGTM this release
1. An OWNER runs `git tag -s $VERSION` and inserts the changelog and pushes the tag with `git push $VERSION`
1. Anyone creates a pull-request updating the vendored version of the code in the Kubelet
1. The release issue is closed
