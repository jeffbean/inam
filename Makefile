PACKAGES := ./...

.DEFAULT_GOAL:=all

VERSION=1.0
COMMIT=$(shell git rev-parse HEAD)
BINARY=inam
TEST_REPORT=tests.xml

# LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.Build=${COMMIT}"

.PHONY: run
run: build
	./cli/cli

.PHONY: all
all: clean format vet build test

.PHONY: build
build: format vet
	@echo "Running go build"
	go build -i ${LDFLAGS} $(PACKAGES)
	go build -i ${LDFLAGS} .

.PHONY: test
test: build
	@echo "Running all tests"
	go test -cover -race $(PACKAGES)

.PHONY: format
format:
	@echo "Running go fmt"
	go fmt $(PACKAGES)

.PHONY: vet
vet:
	@echo "Running go vet"
	go vet $(PACKAGES)

.PHONY: clean
clean:
	@echo "Running clean"
	if [ -f ${BINARY} ]; then rm ${BINARY}; fi


