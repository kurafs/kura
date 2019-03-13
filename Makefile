GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOINSTALL=$(GOCMD) install
BINARY_NAME=kura
BINARY_LINUX=$(BINARY_NAME)_linux

.PHONY: all build clean test install uninstall build-linux

all: test build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v
test:
	$(GOTEST) -v ./...
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_LINUX)
install:
	$(GOINSTALL)
uninstall:
	$(GOCLEAN) -i .

build-linux:
	GOOS=linux $(GOBUILD) -o $(BINARY_LINUX) -v

