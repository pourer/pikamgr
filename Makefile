ifeq "$(GOPATH)" ""
  $(error Please set the environment variable GOPATH before running `make`)
endif

export PATH := $(PATH):$(GOPATH)/bin

BRANCH			:= $(shell git branch | sed --quiet 's/* \(.*\)/\1/p')
GITREV 			:= $(shell git rev-parse --short HEAD)
BUILDTIME 		:= $(shell date '+%F %T %Z')
COMPILERVERSION	:= $(subst go version ,,$(shell go version))
PROJNAME        := pikamgr

define GENERATE_VERSION_CODE
cat << EOF | gofmt > config/version.go
package config

const (
    Version = "$(BRANCH) $(GITREV)"
    Compile = "$(BUILDTIME) $(COMPILERVERSION)"
)
EOF
endef
export GENERATE_VERSION_CODE

PACKAGES := $$(go list ./...| grep -vE 'vendor')
FILES    := $$(find . -name '*.go' | grep -vE 'vendor')

define TEST_COVER
#!/bin/bash

set -e

which gocov >/dev/null || go get -v -u github.com/axw/gocov/gocov

COV_FILE=coverage.txt
COV_TMP_FILE=coverage_tmp.txt

rm -f $$COV_FILE
rm -f $$COV_TMP_FILE
touch $$COV_TMP_FILE

echo "mode: count" > $$COV_FILE

for pkg in $(PACKAGES); do
	go test -v $$pkg -covermode=count -coverprofile=$$COV_TMP_FILE
	tail -n +2 $$COV_TMP_FILE >> $$COV_FILE || (echo "Unable to append coverage for $$pkg" && exit 1)
done

gocov convert $$COV_FILE | gocov report | grep 'Total Coverage'

rm -f $$COV_FILE
rm -f $$COV_TMP_FILE

endef
export TEST_COVER

all: install

install: pika-dashboard pika-fe redis-server
	tar czf $(PROJNAME).tar.gz bin/*
	mv  $(PROJNAME).tar.gz $(GOPATH)/bin/

build: pika-dashboard pika-fe redis-server
	@cp -rf bin $(GOPATH)/bin

deps: generateVer
	@mkdir -p bin

generateVer:
	@echo "$$GENERATE_VERSION_CODE" | bash
	
pika-dashboard: deps
	go build -i -o bin/pika-dashboard ./cmd/dashboard

pika-fe: deps
	go build -i -o bin/pika-fe ./cmd/fe
	@rm -rf bin/assets; cp -rf cmd/fe/assets bin/

redis-server:
	@cp -f extern/redis-3.2.11/redis-benchmark bin/
	@cp -f extern/redis-3.2.11/redis-cli bin/
	@cp -f extern/redis-3.2.11/redis-sentinel bin/

test:
	@echo "$$TEST_COVER" | bash

race:
	go test -v -race $(PACKAGES)

check:
	which golint >/dev/null || go get -v -u github.com/golang/lint/golint
	@echo "vet"
	@go tool vet $(FILES) 2>&1 | awk '{print} END{if(NR>0) {exit 1}}'
	@echo "vet --shadow"
	@go tool vet --shadow $(FILES) 2>&1 | awk '{print} END{if(NR>0) {exit 1}}'
	@echo "golint"
ifdef LINTEXCEPTION
	@golint $(PACKAGES) | grep -vE'$(LINTEXCEPTION)' | awk '{print} END{if(NR>0) {exit 1}}'
else
	@golint $(PACKAGES) | awk '{print} END{if(NR>0) {exit 1}}'
endif

errcheck:
	which errcheck >/dev/null || go get -v -u github.com/kisielk/errcheck
	errcheck -blank $(PACKAGES)

clean:
	@rm -rf bin
	@go clean -i ./...

dev: check test race build

update:
	which glide >/dev/null || curl https://glide.sh/get | sh
	which glide-vc || go get -v -u github.com/sgotti/glide-vc
	rm -r vendor
ifdef PKG
	glide get -v --skip-test $(PKG)
else
	glide update -v -u --skip-test
endif
	@echo "removing test files"
	glide vc --only-code --no-tests --use-lock-file