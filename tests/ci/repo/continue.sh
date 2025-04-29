#!/bin/bash

export OCUROOT_HOME=$(mktemp -d)
echo "OCUROOT_HOME: $OCUROOT_HOME"

ocuroot work continue
ocuroot state diff | xargs -r -n1 ocuroot state apply
ocuroot work trigger