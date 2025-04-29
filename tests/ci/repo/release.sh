#!/bin/bash

export OCUROOT_HOME=$(mktemp -d)
echo "OCUROOT_HOME: $OCUROOT_HOME"

echo "== environments =="
ocuroot release new environments.ocu.star
echo "== release =="
ocuroot release new release.ocu.star
echo "== trigger =="
ocuroot work trigger