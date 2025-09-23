.PHONY: live/server-example install

live/server-example:
	templier

templ:
	templ generate

install: templ
	mkdir -p ~/ocuroot
	go build -o ~/ocuroot/ocuroot ./cmd/ocuroot

test-build: templ
	mkdir -p ./tests/bin
	go build -o ./tests/bin/ocuroot ./cmd/ocuroot

e2e: test-build
	NO_INSTALL=1 ./tests/minimal/test.sh
	NO_INSTALL=1 ./tests/dependencies/test.sh
	NO_INSTALL=1 ./tests/errors/test.sh
	NO_INSTALL=1 ./tests/versioning/test.sh
	NO_INSTALL=1 ./tests/gitstate/test.sh
	NO_INSTALL=1 ./tests/gitstate_shared/test.sh
	NO_INSTALL=1 ./tests/ci/test.sh
	NO_INSTALL=1 ./tests/secrets/test.sh
	NO_INSTALL=1 ./tests/environments/test.sh
	NO_INSTALL=1 ./tests/retries/test.sh
	NO_INSTALL=1 ./tests/validation/test.sh
	NO_INSTALL=1 ./tests/customstate/test.sh
	NO_INSTALL=1 ./tests/cascade/test.sh

