#!/bin/bash

export OCUROOT_HOME=$(mktemp -d)
echo "OCUROOT_HOME: $OCUROOT_HOME"

# Run the omnibus command to perform any outstanding work for this commit
ocuroot work any