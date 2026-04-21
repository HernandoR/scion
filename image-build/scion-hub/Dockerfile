# syntax=docker/dockerfile:1
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Scion Hub Image - standalone server binary with embedded web frontend assets
ARG BASE_IMAGE
FROM ${BASE_IMAGE} AS builder

USER root
WORKDIR /src
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=1

# Build arguments for version info
ARG GIT_COMMIT
ARG BUILD_TIME

# Copy source required for web build + go build
COPY go.mod go.sum ./
COPY cmd/ ./cmd/
COPY pkg/ ./pkg/
COPY web/ ./web/

# Install web dependencies
RUN cd web && npm ci

# Build web assets to embed in Go binary
RUN cd web && npm run build

# Build with version info injected via ldflags
# VCS stamping is disabled since .git is not in the build context;
# commit info is passed explicitly via the GIT_COMMIT build arg
RUN BUILD_TIME="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" && \
    SCION_PKG="github.com/GoogleCloudPlatform/scion/pkg/version" && \
    SCION_LDFLAGS="-s -w -X ${SCION_PKG}.BuildTime=${BUILD_TIME}" && \
    if [ -n "${GIT_COMMIT}" ]; then \
        SCION_LDFLAGS="${SCION_LDFLAGS} -X ${SCION_PKG}.Commit=${GIT_COMMIT}"; \
    fi && \
    go build -buildvcs=false -ldflags="${SCION_LDFLAGS}" -o /usr/local/bin/scion ./cmd/scion/

# Final stage
FROM ${BASE_IMAGE}

USER root
COPY --from=builder /usr/local/bin/scion /usr/local/bin/scion

RUN useradd -m -s /bin/bash -u 1000 scion && \
    mkdir -p /home/scion/.scion && \
    chown -R scion:scion /home/scion

USER scion
WORKDIR /home/scion

EXPOSE 8080 9800 9810

ENTRYPOINT ["scion"]
CMD ["server", "start", "--foreground", "--production", "--enable-hub", "--enable-web"]
