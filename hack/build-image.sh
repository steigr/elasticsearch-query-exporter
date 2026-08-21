#!/usr/bin/env bash
# Builds and (optionally) pushes the multiarch container image.
#
# Usage:
#   hack/build-image.sh [image[:tag]] [--push]
#
# Requires a buildx builder that supports linux/amd64 and linux/arm64, e.g.:
#   docker buildx create --name multiarch-builder --use
set -euo pipefail

IMAGE="${1:-elasticsearch-query-exporter:dev}"
PUSH="${2:-}"

OUTPUT="--load"
if [[ "${PUSH}" == "--push" ]]; then
  OUTPUT="--push"
fi

docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t "${IMAGE}" \
  ${OUTPUT} \
  "$(dirname "$0")/.."
