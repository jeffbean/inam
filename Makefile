PACKAGES := $(shell glide novendor)

.DEFAULT_GOAL:=build

VERSION=1.0
COMMIT=$(shell git rev-parse HEAD)

TEST_REPORT= tests.xml

# LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.Build=${COMMIT}"

.PHONY: run
run: build
	./cli/cli

.PHONY: build
build: format
	@echo "Running go build"
	go build -i ${LDFLAGS} $(PACKAGES)
	go build -i ${LDFLAGS} .

.PHONY: test
test:
	@echo "Running all tests"
	go test -cover -race $(PACKAGES)

.PHONY: install
install:
	glide --version || go get github.com/Masterminds/glide
	glide install

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


