# dist-git Dockerfile
# We cannot use a golang image as building the kubelet requires rsync and that is not present plus there is no easy way
# to install it in the golang image.
FROM openshift/golang-builder:rhel_8_1.18 as build
LABEL stage=build
WORKDIR $REMOTE_SOURCE_DIR/build/

# dos2unix is needed to build CNI plugins
RUN yum install -y dos2unix

WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator
# Copy .git metadata so that we can generate the version for the WMCO binary
COPY $REMOTE_SOURCE/app/.git .git

# Build WMCB
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/windows-machine-config-bootstrapper/
COPY $REMOTE_SOURCE/app/windows-machine-config-bootstrapper/ .
RUN make build

# Build hybrid-overlay
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/ovn-kubernetes/
COPY $REMOTE_SOURCE/app/ovn-kubernetes/ .
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/ovn-kubernetes/go-controller/
RUN make windows

# Build promu utility tool, needed to build the windows_exporter.exe metrics binary
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/promu/
COPY $REMOTE_SOURCE/app/promu/ .
# Explicitly set the $GOBIN path for promu installation
RUN GOBIN=$REMOTE_SOURCE_DIR/build/windows-machine-config-operator/windows_exporter/ go install .

# Build windows_exporter
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/windows_exporter/
COPY $REMOTE_SOURCE/app/windows_exporter/ .
RUN GOOS=windows ./promu build -v

# Build containerd
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/containerd/
COPY $REMOTE_SOURCE/app/containerd/ .
RUN GOOS=windows make bin/containerd.exe

# Build containerd shim
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/hcsshim/
COPY $REMOTE_SOURCE/app/hcsshim/ .
RUN GOOS=windows go build ./cmd/containerd-shim-runhcs-v1

# Build kubelet
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/kubelet/
COPY $REMOTE_SOURCE/app/kubelet/ .
ENV KUBE_BUILD_PLATFORMS windows/amd64
RUN make WHAT=cmd/kubelet

# Build kube-proxy
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/kube-proxy/
COPY $REMOTE_SOURCE/app/kube-proxy/ .
ENV KUBE_BUILD_PLATFORMS windows/amd64
RUN make WHAT=cmd/kube-proxy

# Build azure-cloud-node-manager
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/cloud-provider-azure/
COPY $REMOTE_SOURCE/app/cloud-provider-azure/ .
RUN GOOS=windows go build -o azure-cloud-node-manager.exe ./cmd/cloud-node-manager

# Build CNI plugins
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/containernetworking-plugins/
COPY $REMOTE_SOURCE/app/containernetworking-plugins/ .
ENV CGO_ENABLED=0
RUN ./build_windows.sh

# Build WMCO
WORKDIR $REMOTE_SOURCE_DIR/build/windows-machine-config-operator
# Copy files and directories needed to build the WMCO binary
COPY $REMOTE_SOURCE/app/build build
COPY $REMOTE_SOURCE/app/cmd cmd
COPY $REMOTE_SOURCE/app/controllers controllers
COPY $REMOTE_SOURCE/app/bundle bundle
COPY $REMOTE_SOURCE/app/hack hack
COPY $REMOTE_SOURCE/app/pkg pkg
COPY $REMOTE_SOURCE/app/test test
COPY $REMOTE_SOURCE/app/vendor vendor
COPY $REMOTE_SOURCE/app/version version
COPY $REMOTE_SOURCE/app/go.mod go.mod
COPY $REMOTE_SOURCE/app/go.sum go.sum
COPY $REMOTE_SOURCE/app/Makefile Makefile
COPY $REMOTE_SOURCE/app/tools.go tools.go
COPY $REMOTE_SOURCE/app/bundle.Dockerfile bundle.Dockerfile
COPY $REMOTE_SOURCE/app/.gitignore .gitignore
RUN make build

# Build the operator image with following payload structure
# /payload/
#├── azure-cloud-node-manager.exe
#├── cni
#│   ├── host-local.exe
#│   ├── win-bridge.exe
#│   ├── win-overlay.exe
#│   └── cni-conf-template.json
#├── containerd
#│   ├── containerd.exe
#│   └── containerd-shim-runhcs-v1.exe
#│   └── containerd_conf.toml
#├── hybrid-overlay-node.exe
#├── kube-node
#│   ├── kubelet.exe
#│   └── kube-proxy.exe
#├── powershell
#│   └── wget-ignore-cert.ps1
#│   └── hns.psm1
#├── windows_exporter.exe
#└── wmcb.exe

FROM registry.redhat.io/ubi8/ubi-minimal:latest
LABEL stage=operator
# This label maps to the brew build target
LABEL com.redhat.component="windows-machine-config-operator-container" \
    name="openshift4-wincw/windows-machine-config-operator" \
    summary="OpenShift Windows Machine Config Operator" \
    License="ASL 2.0" \
    io.k8s.display-name="OpenShift Windows Machine Config Operator" \
    io.k8s.description="OpenShift Windows Machine Config Operator" \
    io.openshift.tags="openshift,windows" \
    io.openshift.maintainer.product="OpenShift Container Platform" \
    io.openshift.maintainer.component="Windows Containers" \
    com.redhat.delivery.appregistry="false" \
    version="6.0.0" \
    maintainer="team-winc@redhat.com"

# Install tar so that we do "oc cp" to extract the manifests in the CPaaS pipeline
RUN microdnf -y install tar && microdnf clean all

# Copy wmcb.exe
WORKDIR /payload/
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/windows-machine-config-bootstrapper/wmcb.exe .

# Copy hybrid-overlay-node.exe
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/ovn-kubernetes/go-controller/_output/go/bin/windows/hybrid-overlay-node.exe .

# Copy windows_exporter.exe
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/windows_exporter/windows_exporter.exe .

# Copy azure-cloud-node-manager.exe
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/cloud-provider-azure/azure-cloud-node-manager.exe .

# Copy containerd.exe, containerd-shim-runhcs-v1.exe and containerd config containerd_conf.toml
WORKDIR /payload/containerd/
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/containerd/bin/containerd.exe .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/hcsshim/containerd-shim-runhcs-v1.exe .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/pkg/internal/containerd_conf.toml .

# Copy kubelet.exe and kube-proxy.exe
WORKDIR /payload/kube-node/
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/kubelet/_output/local/bin/windows/amd64/kubelet.exe .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/kube-proxy/_output/local/bin/windows/amd64/kube-proxy.exe .

# Copy CNI plugin binaries and CNI config template cni-conf-template.json
WORKDIR /payload/cni/
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/containernetworking-plugins/bin/host-local.exe .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/containernetworking-plugins/bin/win-bridge.exe .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/containernetworking-plugins/bin/win-overlay.exe .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/pkg/internal/cni-conf-template.json .

# Copy required powershell scripts
WORKDIR /payload/powershell/
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/pkg/internal/wget-ignore-cert.ps1 .
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/pkg/internal/hns.psm1 .

# Copy manifests, metadata and bundle.Dockerfile. They are required to build the bundle image.
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/bundle/manifests /manifests
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/bundle/metadata /metadata
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/bundle.Dockerfile /bundle.Dockerfile

WORKDIR /

ENV OPERATOR=/usr/local/bin/windows-machine-config-operator \
    USER_UID=1001 \
    USER_NAME=windows-machine-config-operator

# install operator binary
COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/build/_output/bin/windows-machine-config-operator ${OPERATOR}

COPY --from=build $REMOTE_SOURCE_DIR/build/windows-machine-config-operator/build/bin /usr/local/bin
RUN  /usr/local/bin/user_setup

ENTRYPOINT ["/usr/local/bin/entrypoint"]

USER ${USER_UID}
