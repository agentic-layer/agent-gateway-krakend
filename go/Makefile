PLUGINS=openai-a2a agentcard-rw

.PHONY: test
test:
	go test -cover ./...

.PHONY: generate
generate: generate-a2a

.PHONY: generate-a2a
generate-a2a:
	mkdir -p lib/models/schema
	curl -sL https://raw.githubusercontent.com/a2aproject/A2A/refs/heads/main/specification/json/a2a.json -o lib/models/schema/a2a.json
	go run github.com/atombender/go-jsonschema \
		--only-models \
		--package=models \
		--output=lib/models/a2a_generated.go \
		lib/models/schema/a2a.json

.PHONY: plugins
plugins: $(PLUGINS)

.PHONY: $(PLUGINS)
$(PLUGINS):
	go get -t ./...
	go build -buildmode=plugin -o ../build/$@.so ./plugin/$@
	go test -cover ./plugin/$@
