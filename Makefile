## Targets
BINARY     := darkhn
CMD        := ./cmd/darkhn
BUILD_DIR  := ./bin

.PHONY: all build test clean docker-build run

all: build

## build – compile the server binary into ./bin/darkhn
build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BUILD_DIR)/$(BINARY) $(CMD)

## test – run all tests with race detection
test:
	go test -race -count=1 ./...

## run – build and start the server (PORT env var optional, default 8080)
run: build
	$(BUILD_DIR)/$(BINARY)

## clean – remove compiled artefacts
clean:
	rm -rf $(BUILD_DIR)

## docker-build – build the Docker image
docker-build:
	docker build -t $(BINARY):latest .
