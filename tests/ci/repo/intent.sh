#!/usr/bin/env bash

export OCUROOT_HOME="$1"
if [ -z "$OCUROOT_HOME" ]; then
  export OCUROOT_HOME=$(mktemp -d)
fi
mkdir -p "$OCUROOT_HOME"

ocuroot work trigger --intent