# dist-git Dockerfile
# We cannot use a golang image as building the kubelet requires rsync and that is not present plus there is no easy way
# to install it in the golang image.
FROM openshift/golang-builder:rhel_8_1.15 as build
LABEL stage=build
RUN yum -y update
RUN yum -y install rsync make go git tar findutils diffutils

## Build WMCO
COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR
WORKDIR $REMOTE_SOURCE_DIR/app
RUN make build

# Build WMCB
WORKDIR $REMOTE_SOURCE_DIR/app/windows-machine-config-bootstrapper/
RUN make build

# Build hybrid-overlay
WORKDIR $REMOTE_SOURCE_DIR/app/ovn-kubernetes/go-controller/
RUN make windows

# Build Kubernetes node binaries
WORKDIR $REMOTE_SOURCE_DIR/app/kubernetes/
ENV KUBE_BUILD_PLATFORMS windows/amd64
RUN make WHAT=cmd/kubelet
RUN make WHAT=cmd/kube-proxy

# Build CNI plugins
WORKDIR $REMOTE_SOURCE_DIR/app/containernetworking-plugins/
ENV CGO_ENABLED=0
RUN ./build_windows.sh

# Build the operator image with following payload structure
# /payload/
#├── cni
#│   ├── flannel.exe
#│   ├── host-local.exe
#│   ├── win-bridge.exe
#│   ├── win-overlay.exe
#│   └── cni-conf-template.json
#├── hybrid-overlay-node.exe
#├── kube-node
#│   ├── kubelet.exe
#│   └── kube-proxy.exe
#├── powershell
#│   └── wget-ignore-cert.ps1
#│   └── hns.psm1
#└── wmcb.exe

FROM registry.redhat.io/ubi8/ubi-minimal:latest
LABEL stage=operator
# This label maps to the brew build target
LABEL com.redhat.component="windows-machine-config-operator-container" \
    name="openshift4/windows-machine-config-operator" \
    summary="OpenShift Windows Machine Config Operator" \
    License="ASL 2.0" \
    io.k8s.display-name="OpenShift Windows Machine Config Operator" \
    io.k8s.description="OpenShift Windows Machine Config Operator" \
    io.openshift.tags="openshift,windows" \
    io.openshift.maintainer.product="OpenShift Container Platform" \
    io.openshift.maintainer.component="Windows Containers" \
    com.redhat.delivery.appregistry="false" \
    version="0.0.1" \
    maintainer="team-winc@redhat.com"

# Install tar so that we do "oc cp" to extract the manifests in the CPaaS pipeline
RUN microdnf -y update
RUN microdnf -y install tar

# Copy wmcb.exe
WORKDIR /payload/
COPY --from=build $REMOTE_SOURCE_DIR/app/windows-machine-config-bootstrapper/wmcb.exe .

# Copy hybrid-overlay-node.exe
COPY --from=build $REMOTE_SOURCE_DIR/app/ovn-kubernetes/go-controller/_output/go/bin/windows/hybrid-overlay-node.exe .

# Copy kubelet.exe and kube-proxy.exe
RUN mkdir /payload/kube-node/
WORKDIR /payload/kube-node/
COPY --from=build $REMOTE_SOURCE_DIR/app/kubernetes/_output/local/bin/windows/amd64/kubelet.exe .
COPY --from=build $REMOTE_SOURCE_DIR/app/kubernetes/_output/local/bin/windows/amd64/kube-proxy.exe .

# Copy CNI plugin binaries and CNI config template cni-conf-template.json
RUN mkdir /payload/cni/
WORKDIR /payload/cni/
COPY --from=build $REMOTE_SOURCE_DIR/app/containernetworking-plugins/bin/flannel.exe .
COPY --from=build $REMOTE_SOURCE_DIR/app/containernetworking-plugins/bin/host-local.exe .
COPY --from=build $REMOTE_SOURCE_DIR/app/containernetworking-plugins/bin/win-bridge.exe .
COPY --from=build $REMOTE_SOURCE_DIR/app/containernetworking-plugins/bin/win-overlay.exe .
COPY --from=build $REMOTE_SOURCE_DIR/app/pkg/internal/cni-conf-template.json .

# Copy required powershell scripts
RUN mkdir /payload/powershell/
WORKDIR /payload/powershell/
COPY --from=build $REMOTE_SOURCE_DIR/app/pkg/internal/wget-ignore-cert.ps1 .
COPY --from=build $REMOTE_SOURCE_DIR/app/pkg/internal/hns.psm1 .

# Copy manifests, metadata and bundle.Dockerfile. They are required to build the bundle image.
COPY --from=build $REMOTE_SOURCE_DIR/app/deploy/olm-catalog/windows-machine-config-operator/manifests /manifests
COPY --from=build $REMOTE_SOURCE_DIR/app/deploy/olm-catalog/windows-machine-config-operator/metadata /metadata
COPY --from=build $REMOTE_SOURCE_DIR/app/bundle.Dockerfile /bundle.Dockerfile

WORKDIR /

ENV OPERATOR=/usr/local/bin/windows-machine-config-operator \
    USER_UID=1001 \
    USER_NAME=windows-machine-config-operator

# install operator binary
COPY --from=build $REMOTE_SOURCE_DIR/app/build/_output/bin/windows-machine-config-operator ${OPERATOR}

COPY --from=build $REMOTE_SOURCE_DIR/app/build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
