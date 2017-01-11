# Rktnetes Test Infra

This repo contains configuration used to manage e2e testing of the rktnetes project.

It is based largely on the work present in the
[test-infra](https://github.com/kubernetes/test-infra) repo used for most of
Kubernetes' testing.

Ideally, this testing would be included in that repository and maintained in a
similar manner to them. Pending its acceptance there and results being reliable
enough that they may live there, they'll be run via this repository.

## Status

Currently the configuration in this repo specifies jobs run on CoreOS's jenkins
infrastructure against the `kubelet/rkt` version of rktnetes. It does not e2e
against the rktlet code in this repo.

The test result artifacts are published to the
[rktnetes-jenkins](https://console.cloud.google.com/storage/browser/rktnetes-jenkins/logs/)
bucket where they are then further consumed by projects like the
[testgrid](https://k8s-testgrid.appspot.com/rkt) as configured
[here](https://github.com/kubernetes/test-infra/blob/3824bde1c2633961db012c19215105d58006ad3f/testgrid/config/config.yaml#L833-L834).
