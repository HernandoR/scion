#!/bin/bash
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

set -euo pipefail

# Trigger Cloud Build for scion images.
#
# Usage:
#   trigger-cloudbuild.sh [--project <project>] [--registry <registry>] [target]
#
# Examples:
#   trigger-cloudbuild.sh                              # defaults: project from gcloud, common target
#   trigger-cloudbuild.sh --project my-gcp-project all
#   trigger-cloudbuild.sh --registry us-central1-docker.pkg.dev/myproj/scion harnesses

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
cd "${REPO_ROOT}"

PROJECT=""
REGISTRY=""
TARGET=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --project)  PROJECT="$2"; shift 2 ;;
    --registry) REGISTRY="$2"; shift 2 ;;
    -h|--help)
      echo "Usage: $(basename "$0") [--project <project>] [--registry <registry>] [target]"
      echo ""
      echo "Targets: common (default), all, core-base, scion-base, hub, harnesses"
      echo ""
      echo "Options:"
      echo "  --project <project>    GCP project (default: \$GCLOUD_PROJECT or gcloud config)"
      echo "  --registry <registry>  Override \$_REGISTRY substitution in Cloud Build"
      exit 0
      ;;
    -*) echo "Unknown option: $1"; exit 1 ;;
    *)  TARGET="$1"; shift ;;
  esac
done

TARGET="${TARGET:-common}"

# Resolve project
if [[ -z "${PROJECT}" ]]; then
  PROJECT="${GCLOUD_PROJECT:-}"
fi
if [[ -z "${PROJECT}" ]]; then
  PROJECT="$(gcloud config get-value project 2>/dev/null)" || true
fi
if [[ -z "${PROJECT}" ]]; then
  echo "Error: Could not determine GCP project."
  echo "Set --project, \$GCLOUD_PROJECT, or run 'gcloud config set project <project>'."
  exit 1
fi

SHORT_SHA=$(git rev-parse --short HEAD)
COMMIT_SHA=$(git rev-parse HEAD)

# Build substitutions
SUBSTITUTIONS="_SHORT_SHA=${SHORT_SHA},_COMMIT_SHA=${COMMIT_SHA}"
if [[ -n "${REGISTRY}" ]]; then
  SUBSTITUTIONS="${SUBSTITUTIONS},_REGISTRY=${REGISTRY}"
fi

case "${TARGET}" in
  common)
    echo "Submitting common build (scion-base + scion-hub -> harnesses) to Cloud Build..."
    CONFIG="image-build/cloudbuild-common.yaml"
    ;;
  all)
    echo "Submitting full build (core-base -> scion-base -> harnesses) to Cloud Build..."
    CONFIG="image-build/cloudbuild.yaml"
    ;;
  core-base)
    echo "Submitting core-base build to Cloud Build..."
    CONFIG="image-build/cloudbuild-core-base.yaml"
    ;;
  scion-base)
    echo "Submitting scion-base build to Cloud Build..."
    CONFIG="image-build/cloudbuild-scion-base.yaml"
    ;;
  hub)
    echo "Submitting scion-hub build to Cloud Build..."
    CONFIG="image-build/cloudbuild-hub.yaml"
    ;;
  harnesses)
    echo "Submitting harnesses build to Cloud Build..."
    CONFIG="image-build/cloudbuild-harnesses.yaml"
    ;;
  *)
    echo "Unknown target: ${TARGET}"
    echo "Usage: $(basename "$0") [--project <project>] [--registry <registry>] [target]"
    echo ""
    echo "Targets:"
    echo "  common      - Rebuild scion-base + harnesses, skip core-base (default)"
    echo "  all         - Full rebuild of all images including core-base"
    echo "  core-base   - Build only core-base (foundation tools)"
    echo "  scion-base  - Build only scion-base (uses existing core-base:latest)"
    echo "  harnesses   - Build only harnesses (uses existing scion-base:latest)"
    exit 1
    ;;
esac

gcloud builds submit --async \
  --project="${PROJECT}" \
  --substitutions="${SUBSTITUTIONS}" \
  --config="${CONFIG}" .

echo ""
echo "Build submitted. View progress at:"
echo "  https://console.cloud.google.com/cloud-build/builds?project=${PROJECT}"
