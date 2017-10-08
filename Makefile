BINARY = inam

VET_REPORT = vet.report
TEST_REPORT = tests.xml
GOARCH = amd64

VERSION=1.0
COMMIT=$(shell git rev-parse HEAD)

LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.Build=${COMMIT}"

build:
	go build ${} -o ${BINARY}

clean: 
	if [ -f ${BINARY} ]; then rm ${BINARY}; fi

.PHONY: clean 

