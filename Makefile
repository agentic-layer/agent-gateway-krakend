ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: all
all: image

.PHONY: image
image:
	docker build --tag agent-gateway-krakend .

.PHONY: run
run: image
	docker run -p 8080:8080 -p 9090:9090 \
		-v $(PWD)/test/krakend.json:/etc/krakend/krakend.json \
		-e OPENAI_API_KEY=$(OPENAI_API_KEY) agent-gateway-krakend
