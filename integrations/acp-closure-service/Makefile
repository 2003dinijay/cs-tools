BINARY := acp-closure

vet:
	go vet ./...

test: vet
	go test -race ./...

build: test
	go build -o $(BINARY) ./cmd/acp-closure

clean:
	rm -f $(BINARY)

.PHONY: vet test build clean
