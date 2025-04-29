#!/bin/bash

export OCUROOT_HOME=$(mktemp -d)
echo "OCUROOT_HOME: $OCUROOT_HOME"

echo "== environments =="
ocuroot release new environments.ocu.star
echo "== release =="
if [ $(( $(cat message-backend.txt) % $(cat message-frontend.txt) )) -eq 0 ]; then
    echo "== frontend =="
    ocuroot release new frontend.ocu.star
fi

echo "== backend =="
echo 
ocuroot release new backend.ocu.star
echo "== trigger =="
ocuroot work trigger