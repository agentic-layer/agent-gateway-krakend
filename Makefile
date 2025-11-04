ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: all
all: image

.PHONY: generate
generate:
	$(MAKE) -C ./go generate

.PHONY: plugins
plugins:
	$(MAKE) -C ./go plugins

.PHONY: test
test:
	$(MAKE) -C ./go test

.PHONY: image
image:
	docker build --tag agent-gateway-krakend .
