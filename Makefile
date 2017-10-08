BINARY = inam

VET_REPORT = vet.report
TEST_REPORT = tests.xml
GOARCH = amd64

VERSION?=?
COMMIT=$(shell git rev-parse HEAD)
BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

build:
	go build -o ${BINARY}

clean: 
	if [ -f ${BINARY} ]; then rm ${BINARY}; fi

.PHONY: clean 

