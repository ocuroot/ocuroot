#!/bin/bash

export OCUROOT_HOME=$(mktemp -d)
echo "OCUROOT_HOME: $OCUROOT_HOME"

ocuroot work trigger --intent