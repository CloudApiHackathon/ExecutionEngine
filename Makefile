# Go parameters
GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_CLEAN=$(GO_CMD) clean
GO_VET=$(GO_CMD) vet
GO_TEST=$(GO_CMD) test

# Binary names
BINARY_NAME=ExecutionEngine

# Directories
PROTOBUF_DIR=proto
DIST_DIR=dist

# Targets
all: debug

.PHONY: tools
tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

protobuf: $(wildcard $(PROTOBUF_DIR)/*/*.proto)
	protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative $^

debug: protobuf
	$(GO_VET) -tags="debug"
	$(GO_TEST) -tags="debug"
	$(GO_BUILD) -tags="debug" -o $(DIST_DIR)/$(BINARY_NAME)-debug .

release: protobuf
	$(GO_VET) -tags="release"
	$(GO_TEST) -tags="release"
	$(GO_BUILD) -tags="release" -ldflags="-s -w" -o $(DIST_DIR)/$(BINARY_NAME)-release .

.PHONY: clean
clean:
	$(GO_CLEAN)
	rm -rf $(DIST_DIR)
	rm -rf proto/*.pb.go
