# Fetch or build all required binaries
FROM golang:1.27.1 as builder

ARG VERSION_REF
RUN test -n "${VERSION_REF}"

ENV KUBECTL_VERSION "v1.37.0"
ENV KUBECTL_SHA512_SUM "ff61f94cf73281e24b8b7f16bd7ae15a928ef886399a2e8a360c170828babade73aeb284d37c596fc6c7dc68876b1550b714c1fc3ac4263adb8aef9e30a7c42f"

RUN apt-get update && apt-get install --yes \
    curl \
    wget

RUN wget -q https://dl.k8s.io/${KUBECTL_VERSION}/kubernetes-client-linux-amd64.tar.gz && \
    echo "${KUBECTL_SHA512_SUM} kubernetes-client-linux-amd64.tar.gz" | sha512sum -c && \
    tar -xvzf kubernetes-client-linux-amd64.tar.gz && \
    cp kubernetes/client/bin/kubectl /usr/local/bin

ENV SRC github.com/segmentio/terraform-provider-kubeapply

COPY . /go/src/${SRC}
WORKDIR /go/src/${SRC}

ENV CGO_ENABLED=0
ENV GO111MODULE=on

RUN make terraform-provider-kubeapply VERSION_REF=${VERSION_REF} && \
    cp build/terraform-provider-kubeapply /usr/local/bin
RUN make kadiff VERSION_REF=${VERSION_REF} && \
    cp build/kadiff /usr/local/bin

# Copy into final image
FROM ubuntu:26.04

RUN apt-get update && apt-get install --yes curl git

COPY --from=builder \
    /usr/local/bin/kubectl \
    /usr/local/bin/terraform-provider-kubeapply \
    /usr/local/bin/kadiff \
    /usr/local/bin/
