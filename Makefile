.PHONY: live/server-example install

live/server-example:
	templier

templ:
	templ generate

install: templ
	mkdir -p ~/ocuroot
	go build -o ~/ocuroot/ocuroot ./cmd/ocuroot

e2e: install
	NO_INSTALL=1 ./tests/minimal/test.sh
	NO_INSTALL=1 ./tests/dependencies/test.sh
	NO_INSTALL=1 ./tests/errors/test.sh
	NO_INSTALL=1 ./tests/versioning/test.sh
	NO_INSTALL=1 ./tests/gitstate/test.sh
	NO_INSTALL=1 ./tests/ci/test.sh
	NO_INSTALL=1 ./tests/secrets/test.sh

