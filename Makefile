export GOPATH     := $(abspath ../..)
export NEX        := $(abspath ../../bin/nex)

all: $(NEX) test

$(NEX): main.go nex.go
	go fmt nex
	go install nex

test: $(NEX) $(shell find test -type f)
	go fmt nex nex/test
	go test nex nex/test

clean:
	rm -f $(NEX)

.PHONY: all test clean
