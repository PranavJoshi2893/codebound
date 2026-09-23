
BINARY_NAME = codebound

build:
	@go build -o bin/$(BINARY_NAME) main.go

run: build
	@./bin/$(BINARY_NAME)

clean:
	@rm -rf bin/
	@go clean
