#!/bin/sh

cd "$(dirname "${BASH_SOURCE[0]}")"
cd ../../

go run ./cmd/ocuroot "$@"
