#!/usr/bin/env bash
# Build and push the herm base Docker image for multiple architectures.
# Usage: ./build-push-image.sh [--dry-run]

set -euo pipefail

# Read the image tag from config.go (single source of truth)
TAG=$(grep 'const hermImageTag' cmd/herm/config.go | sed 's/.*"\(.*\)".*/\1/')

if [ -z "$TAG" ]; then
  echo "Error: could not read hermImageTag from cmd/herm/config.go" >&2
  exit 1
fi

IMAGE="aduermael/herm:${TAG}"
PLATFORMS="linux/amd64,linux/arm64"

echo "Image:     ${IMAGE}"
echo "Platforms: ${PLATFORMS}"

if [ "${1:-}" = "--dry-run" ]; then
  echo "(dry-run) Would run:"
  echo "  docker buildx build --platform ${PLATFORMS} -t ${IMAGE} --push ."
  exit 0
fi

# Ensure a buildx builder with multi-arch support exists
BUILDER="herm-multiarch"
if ! docker buildx inspect "$BUILDER" &>/dev/null; then
  echo "Creating buildx builder: ${BUILDER}"
  docker buildx create --name "$BUILDER" --driver docker-container --use
else
  docker buildx use "$BUILDER"
fi

docker buildx build --platform "$PLATFORMS" -t "$IMAGE" --push .

echo "Pushed ${IMAGE} for ${PLATFORMS}"
