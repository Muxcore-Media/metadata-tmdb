.PHONY: build test lint clean fmt tidy proto docker docker-push ci help

GO ?= go
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.0.0-dev")
LDFLAGS ?= -s -w -X main.version=$(VERSION)
BINARY ?= metadata-tmdb

build:
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) ./cmd/module

test:
	$(GO) test -race -count=1 -timeout 60s ./...

lint:
	golangci-lint run --timeout 120s ./...

clean:
	rm -f $(BINARY)
	rm -f cmd/module/module
	rm -rf dist/

fmt:
	$(GO) fmt ./...

tidy:
	$(GO) mod tidy

# Deprecated proto/metadatav1 copy (consumed by media-movies). Generator versions
# match the committed headers; needs protoc 34.1 on PATH. Keep tags identical to
# contracts-metadata for every message this server returns.
PROTOC_GEN_GO_VERSION ?= v1.33.0
PROTOC_GEN_GO_GRPC_VERSION ?= v1.3.0
proto:
	$(GO) install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	$(GO) install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	bin="$$($(GO) env GOBIN)"; [ -n "$$bin" ] || bin="$$($(GO) env GOPATH)/bin"; \
	PATH="$$bin:$$PATH" protoc \
		--go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		-I . proto/metadatav1/metadata.proto

docker:
	docker build -t ghcr.io/muxcore-media/$(BINARY):$(VERSION) .
	docker tag ghcr.io/muxcore-media/$(BINARY):$(VERSION) ghcr.io/muxcore-media/$(BINARY):latest

docker-push: docker
	docker push ghcr.io/muxcore-media/$(BINARY):$(VERSION)
	docker push ghcr.io/muxcore-media/$(BINARY):latest

ci: lint test build

help:
	@echo "Targets:"
	@echo "  build       - compile the module binary"
	@echo "  test        - run tests with race detection"
	@echo "  lint        - golangci-lint"
	@echo "  clean       - remove build artifacts"
	@echo "  fmt         - format Go source"
	@echo "  tidy        - go mod tidy"
	@echo "  proto       - regenerate proto/metadatav1 (protoc 34.1)"
	@echo "  docker      - build Docker image"
	@echo "  docker-push - build and push Docker image"
	@echo "  ci          - lint + test + build"
