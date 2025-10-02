.PHONY: test

PLUGINS=openai-a2a

test:
	go test -cover ./...

plugins: $(PLUGINS)
$(PLUGINS):
	go get -t ./...
	go build -buildmode=plugin -o ../build/$@.so ./plugin/$@
	go test -cover ./plugin/$@
