.PHONY: build test lint fmt vet run docker-build clean

build:
	go build -o slinky .

test:
	go test ./... -race

vet:
	go vet ./...

fmt:
	gofmt -l .

lint: vet
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "gofmt needs to be run on:"; \
		gofmt -l .; \
		exit 1; \
	fi

run:
	go run .

docker-build:
	docker build -t slinky .

clean:
	rm -f slinky coverage.out
