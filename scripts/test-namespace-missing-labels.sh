#!/usr/bin/env bash

set -Eeuo pipefail

# Pass a namespace name as the first argument, or use a unique test name.
namespace="${1:-ns-missing-psp-test-$(date +%s)}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required" >&2
  exit 1
fi

echo "Creating namespace without PSP/PSS labels: ${namespace}"
kubectl create namespace "${namespace}"
echo "Created namespace: ${namespace}"
