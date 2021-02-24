# Current Operator version
VERSION ?= 0.0.1
REGISTRY ?= kind:5000
RHEL ?= false
ifeq ($(RHEL), true)
SOSREPORT_IMG ?= ${REGISTRY}/sosreport-redhat-toolbox:$(VERSION)
SOSREPORT_CONTAINER_LOCATION=containers/sosreport-redhat-toolbox
else
SOSREPORT_IMG ?= ${REGISTRY}/sosreport-centos:$(VERSION)
SOSREPORT_CONTAINER_LOCATION=containers/sosreport-centos
endif
OPERATOR_IMG ?= ${REGISTRY}/sosreport-operator:$(VERSION)
BUNDLE_IMG ?= ${REGISTRY}/sosreport-operator-bundle:$(VERSION)
INDEX_IMG ?= ${REGISTRY}/sosreport-operator-index:${VERSION}

# Options for make deploy-examples
SIMULATION_MODE ?= true
STORAGE_CLASS ?= ""
IMAGE_PULL_POLICY ?= ""
UPLOAD_METHOD ?= "none"
NFS_SHARE ?= "kind:/nfs"

# Options for tests
USE_EXISTING_CLUSTER=false

# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions={v1},trivialVersions=true"
# CRD_OPTIONS ?= "crd:trivialVersions=true"  # for v1betav1

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager

# Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	mkdir -p $(ENVTEST_ASSETS_DIR)
	test -f $(ENVTEST_ASSETS_DIR)/setup-envtest.sh || curl -sSLo $(ENVTEST_ASSETS_DIR)/setup-envtest.sh https://raw.githubusercontent.com/kubernetes-sigs/controller-runtime/v0.6.3/hack/setup-envtest.sh
	source $(ENVTEST_ASSETS_DIR)/setup-envtest.sh; fetch_envtest_tools $(ENVTEST_ASSETS_DIR); setup_envtest_env $(ENVTEST_ASSETS_DIR); SOSREPORT_IMG=${SOSREPORT_IMG} USE_EXISTING_CLUSTER=${USE_EXISTING_CLUSTER} go test ./... -coverprofile cover.out

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${OPERATOR_IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

undeploy:
	$(KUSTOMIZE) build config/default | kubectl delete -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image with docker
docker-build: test
	docker build . -t ${OPERATOR_IMG}

# Push the docker image
docker-push:
	docker push ${OPERATOR_IMG}

# Build the docker image with buildah
podman-build: test
	buildah bud --format docker -t ${OPERATOR_IMG} .

# Push the docker image
podman-push:
	podman push ${OPERATOR_IMG}

podman-copy-sosreport-scripts:
	rm -Rf ${SOSREPORT_CONTAINER_LOCATION}/scripts ; \
	cp -a containers/scripts ${SOSREPORT_CONTAINER_LOCATION}

# Build the docker image with buildah
podman-build-sosreport: podman-copy-sosreport-scripts
	cd ${SOSREPORT_CONTAINER_LOCATION} && buildah bud --format docker -t ${SOSREPORT_IMG} .

# Push the docker image
podman-push-sosreport:
	podman push ${SOSREPORT_IMG}

deploy-test-deployment:
	SOSREPORT_IMG=${SOSREPORT_IMG} bash test-environments/test-deployment/deploy.sh

deploy-examples:
	rm -Rf /tmp/samples && \
	cp -a ./config/samples/ /tmp/samples && \
	sed -i "s#^  simulation-mode:.*#  simulation-mode: \"${SIMULATION_MODE}\"#" /tmp/samples/configmap-sosreport-global-configuration.yaml && \
	sed -i "s#^  sosreport-image:.*#  sosreport-image: \"${SOSREPORT_IMG}\"#" /tmp/samples/configmap-sosreport-global-configuration.yaml && \
	sed -i "s#^  pvc-storage-class:.*#  pvc-storage-class: \"${STORAGE_CLASS}\"#" /tmp/samples/configmap-sosreport-global-configuration.yaml &&\
	sed -i "s#^  image-pull-policy:.*#  image-pull-policy: \"${IMAGE_PULL_POLICY}\"#" /tmp/samples/configmap-sosreport-global-configuration.yaml &&\
	sed -i "s#^  nfs-share:.*#  nfs-share: \"${NFS_SHARE}\"#" /tmp/samples/configmap-sosreport-upload-configuration.yaml &&\
	sed -i "s#^  upload-method:.*#  upload-method: \"${UPLOAD_METHOD}\"#" /tmp/samples/configmap-sosreport-upload-configuration.yaml &&\
	kubectl apply -f /tmp/samples/configmap-sosreport-global-configuration.yaml && \
	kubectl apply -f /tmp/samples/configmap-sosreport-upload-configuration.yaml && \
	kubectl apply -f /tmp/samples/secret-sosreport-upload-secret.yaml && \
	kubectl apply -f /tmp/samples/support_v1alpha1_sosreport.yaml && \
	kubectl get namespaces | grep -q openshift && oc adm policy add-scc-to-user privileged -z default 

undeploy-examples:
	kubectl delete -f /tmp/samples/configmap-sosreport-global-configuration.yaml && \
	kubectl delete -f /tmp/samples/configmap-sosreport-upload-configuration.yaml && \
	kubectl delete -f /tmp/samples/secret-sosreport-upload-secret.yaml && \
	kubectl delete -f /tmp/samples/support_v1alpha1_sosreport.yaml 

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests kustomize
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(OPERATOR_IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

# Build the bundle image.
.PHONY: bundle-build
bundle-build-podman:
	buildah bud -f bundle.Dockerfile -t $(BUNDLE_IMG) .

bundle-push-podman:
	podman push $(BUNDLE_IMG)

bundle-validate:
	operator-sdk bundle validate $(BUNDLE_IMG)

bundle-validate-podman:
	operator-sdk bundle validate -b podman $(BUNDLE_IMG)

opm:
	cd ~ ; \
	go get github.com/operator-framework/operator-registry ; \
	cd $(GOPATH)/src/github.com/operator-framework/operator-registry ; \
	make ; \
	cp bin/opm /usr/local/bin/opm

index-build:
	opm index add --bundles ${BUNDLE_IMG} --tag ${INDEX_IMG}

index-push-podman:
	podman push ${INDEX_IMG}
