module github.com/tektoncd/pipeline

go 1.13

replace github.com/google/go-containerregistry/pkg/authn/k8schain => github.com/imjasonh/go-containerregistry/pkg/authn/k8schain v0.0.0-20220119194151-b5af5f560612

require (
	github.com/cloudevents/sdk-go/v2 v2.5.0
	github.com/containerd/containerd v1.5.8
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.5.6
	github.com/google/go-containerregistry v0.8.1-0.20220110151055-a61fd0a8e2bb
	github.com/google/go-containerregistry/pkg/authn/k8schain v0.0.0-20220114205711-890d5b362eb8
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/golang-lru v0.5.4
	github.com/jenkins-x/go-scm v1.10.10
	github.com/mitchellh/go-homedir v1.1.0
	github.com/opencontainers/image-spec v1.0.3-0.20211202222133-eacdcc10569b
	github.com/pkg/errors v0.9.1
	github.com/tektoncd/plumbing v0.0.0-20211012143332-c7cc43d9bc0c
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.19.1
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	gomodules.xyz/jsonpatch/v2 v2.2.0
	k8s.io/api v0.22.5
	k8s.io/apimachinery v0.22.5
	k8s.io/client-go v0.22.5
	k8s.io/code-generator v0.22.5
	k8s.io/klog v1.0.0
	k8s.io/kube-openapi v0.0.0-20220114203427-a0453230fd26
	knative.dev/pkg v0.0.0-20220104185830-52e42b760b54
)

require (
	cloud.google.com/go/compute v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go v61.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.11 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.14.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.11.0 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20211215200129-69c85dc22db6 // indirect
	github.com/emicklei/go-restful v2.15.0+incompatible // indirect
	github.com/go-logr/logr v1.2.2 // indirect
	github.com/google/go-containerregistry/pkg/authn/kubernetes v0.0.0-20220114205711-890d5b362eb8 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce // indirect
	golang.org/x/net v0.0.0-20220114011407-0dd24b26b47d // indirect
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9 // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/time v0.0.0-20211116232009-f0f3c7e86c11 // indirect
	k8s.io/klog/v2 v2.40.1 // indirect
	k8s.io/utils v0.0.0-20211208161948-7d6a63dca704 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
)
