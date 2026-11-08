BINARY_NAME = codebound

.PHONY: build run clean test vet check test-race test-browser bench deadcode

build:
	@go build -o "bin/$(BINARY_NAME)" .

run: build
	@"./bin/$(BINARY_NAME)"

clean:
	@rm -f -- "bin/$(BINARY_NAME)"
	@go clean

test:
	@go test ./...

vet:
	@go vet ./...

check: test vet

test-race:
	@CGO_ENABLED=1 go test -race ./...

test-browser:
	@CODEBOUND_BROWSER_TESTS=1 go test ./internal/calltree/viewer -run TestBrowserGraphNavigation -count=1

bench:
	@go test ./internal/calltree -run '^$$' -bench . -benchmem

deadcode:
	@go run golang.org/x/tools/cmd/deadcode@v0.52.0 -test ./...
