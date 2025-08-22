#!/bin/bash

export OCUROOT_HOME="$1"
if [ -z "$OCUROOT_HOME" ]; then
  export OCUROOT_HOME=$(mktemp -d)
fi
mkdir -p "$OCUROOT_HOME"

# Run the omnibus command to perform any outstanding work for this commit
ocuroot work any