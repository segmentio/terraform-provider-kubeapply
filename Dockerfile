# Fetch or build all required binaries
FROM 528451384384.dkr.ecr.us-west-2.amazonaws.com/segment-golang:1.17.3 as builder

ARG VERSION_REF
RUN test -n "${VERSION_REF}"

ENV KUBECTL_VERSION "v1.20.7"
ENV KUBECTL_SHA512_SUM "e7bac0324907e48fba1bb9cf0eea3a68f9645591a6e09c6f0af36f3bead88765d85039f0114fae41697a7101df90cf02a18628ef677c7e5e41f2f14c24e6046e"

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
FROM ubuntu:20.04

RUN apt-get update && apt-get install --yes curl git

COPY --from=builder \
    /usr/local/bin/kubectl \
    /usr/local/bin/terraform-provider-kubeapply \
    /usr/local/bin/kadiff \
    /usr/local/bin/
