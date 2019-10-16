export GOPATH     := $(abspath ../../../..)
export NEX        := $(abspath ../../../../bin/nex)

all: $(NEX) test

$(NEX): main.go nex.go
	go fmt github.com/blynn/nex
	go install github.com/blynn/nex

test: $(NEX) $(shell find test -type f)
	go fmt github.com/blynn/nex github.com/blynn/nex/test
	go test github.com/blynn/nex github.com/blynn/nex/test

clean:
	rm -f $(NEX)

.PHONY: all test clean
