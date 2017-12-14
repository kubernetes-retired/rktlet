# rktlet release guide

## Release cycle

This section describes the typical release cycle of rktlet:

1. A GitHub [milestone](https://github.com/kubernetes-incubator/rktlet/milestones) describes the issues that will be addressed for a given release. Releases occur on an as-needed basis.
2. Changes are submitted for review in the form of a GitHub Pull Request (PR). Each PR undergoes review and must pass continuous integration (CI) tests before being accepted and merged into the main line of rktlet source code.

## Release Process

This section shows how to perform a release of rktlet. Only parts of the procedure are automated; this is somewhat intentional (manual steps for sanity checking) but it can probably be further scripted, please help. The following example assumes we're going from version 0.1.0 (v0.1.0) to 0.2.0 (v0.2.0).

Let's get started:

* Start at the relevant milestone on GitHub (e.g. https://github.com/kubernetes-incubator/rktlet/milestones/v0.2.0): ensure all referenced issues are closed (or moved elsewhere, if they're not done)
* Update the [roadmap](ROADMAP.md) to remove the release you're performing, if necessary
* Branch from the latest master, make sure your git status is clean
* Ensure the build is clean!
 * `git clean -ffdx && make` should work
* Update the [release notes](CHANGELOG.md). Try to capture most of the salient changes since the last release, but don't go into unnecessary detail (better to link/reference the documentation wherever possible)
* File a PR and get a LGTM from all the [OWNERS](OWNERS). This is useful to a) sanity check the diff, and b) be very explicit/public that a release is happening
* Ensure the CI on the release PR is green!
* Merge the PR
* Close the milestone on GitHub

Now let's build the release:

* Check out the merge commit from the release PR
* Add a signed tag with `git tag -s $VERSION -m $VERSION`. For this example: `git tag -s v0.2.0 -m v0.2.0`
* Build the binary inside rkt: `make build-in-rkt`
* Sanity check the binary version: For this example `./bin/container/rktlet --version` should say `rktlet version: v0.2.0`
* Push the tag to GitHub: `git push --tags`

Now we switch to the GitHub web UI to conduct the release:

* Start a [new release](https://github.com/kubernetes-incubator/rktlet/releases/new) on GitHub
* Fill the tag and release title fields. For this example: Tag "v0.2.0", release title "v0.2.0"
* Copy-paste the release notes you added earlier in the [changelog](CHANGELOG.md)
* You can also add a little more detail and polish to the release notes here if you wish, as it is more targeted towards users (vs the changelog being more for developers); use your best judgement and see previous releases on GH for examples.
* Generate the release artifact. This is a simple tarball:

```
export RKTLETVER="0.2.0"
export NAME="rktlet-v$RKTLETVER"
mkdir $NAME
cp bin/container/rktlet $NAME/
sudo chown -R root:root $NAME/
tar czvf $NAME.tar.gz --numeric-owner $NAME/
```

* Sign the release artifact:

```
gpg --sign --detach --armor $NAME.tar.gz
```

* Attach the release artifact and its signature to the release
* Once signed and uploaded, double-check that the artifact and signature are on GitHub.
* Publish the release!
* Clean your git tree: `sudo git clean -ffdx`
