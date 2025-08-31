#!/usr/bin/env bash

export OCUROOT_HOME="$1"
if [ -z "$OCUROOT_HOME" ]; then
  export OCUROOT_HOME=$(mktemp -d)
fi
mkdir -p "$OCUROOT_HOME"

echo "== environments =="
ocuroot release new environments.ocu.star
echo "== release =="
ocuroot release new release.ocu.star
echo "== trigger =="
ocuroot work trigger