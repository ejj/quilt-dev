export GO15VENDOREXPERIMENT=1
PACKAGES=$(shell govendor list -no-status +local)
NOVENDOR=$(shell find . -path -prune -o -path ./vendor -prune -o -name '*.go' -print)
LINE_LENGTH_EXCLUDE=./api/pb/pb.pb.go \
		    ./cluster/amazon/client/mocks/% \
		    ./cluster/cloudcfg/template.go \
		    ./cluster/digitalocean/client/mocks/% \
		    ./cluster/google/client/mocks/% \
		    ./cluster/machine/amazon.go \
		    ./cluster/machine/google.go \
		    ./minion/network/link_test.go \
		    ./minion/ovsdb/mock_transact_test.go \
		    ./minion/ovsdb/mocks/Client.go \
		    ./minion/pb/pb.pb.go \
		    ./node_modules/% \
		    ./quilt-tester/tests/zookeeper/vendor/% \
		    ./stitch/bindings.js.go

JS_LINT_COMMAND = node_modules/eslint/bin/eslint.js -c stitch/eslint.conf \
                  stitch/ quilt-tester/
REPO = quilt
DOCKER = docker
SHELL := /bin/bash

all:
	cd -P . && go build .

install:
	cd -P . && go install .

gocheck:
	govendor test $$(govendor list -no-status +local | \
		grep -vE github.com/quilt/quilt/"quilt-tester|scripts")

javascript-check:
	npm test

check: gocheck javascript-check

clean:
	govendor clean -x +local
	rm -f *.cov.coverprofile cluster/*.cov.coverprofile minion/*.cov.coverprofile
	rm -f *.cov.html cluster/*.cov.html minion/*.cov.html
	rm quilt_linux quilt_darwin

linux:
	cd -P . && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o quilt_linux .

darwin:
	cd -P . && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o quilt_darwin .

release: linux darwin

COV_SKIP= /api/client/mocks \
	  /api/pb \
	  /cluster/amazon/client/mocks \
	  /cluster/digitalocean/client/mocks \
	  /cluster/google/client/mocks \
	  /cluster/provider/mocks \
	  /constants \
	  /minion/network/mocks \
	  /minion/nl \
	  /minion/nl/nlmock \
	  /minion/ovsdb/mocks \
	  /minion/pb \
	  /minion/pprofile \
	  /minion/supervisor/images \
	  /quilt-tester/% \
	  /quiltctl/ssh/mocks \
	  /quiltctl/testutils \
	  /scripts \
	  /scripts/blueprints-tester \
	  /scripts/blueprints-tester/tests \
	  /scripts/format \
	  /version

go-coverage:
	for package in $(filter-out $(COV_SKIP), $(subst github.com/quilt/quilt,,$(PACKAGES))) ; do \
	    go test -coverprofile=.$$package.cov .$$package && \
	    go tool cover -html=.$$package.cov -o .$$package.html ; \
	done
	echo "" > coverage.txt
	for f in $^ ; do \
	    cat .$$f >> coverage.txt ; \
	done

js-coverage:
	npm run-script cov > bindings.lcov

coverage: go-coverage js-coverage

format:
	gofmt -w -s $(NOVENDOR)
	$(JS_LINT_COMMAND) --fix

scripts/format/format: scripts/format/format.go
	cd scripts/format && go build format.go

build-blueprints-tester: scripts/blueprints-tester/*
	cd scripts/blueprints-tester && go build .

check-blueprints: build-blueprints-tester
	scripts/blueprints-tester/blueprints-tester

# lint checks the format of all of our code. This command should not make any
# changes to fix incorrect format; it should only check it. Code to update the
# format should go under the format target.
lint: golint jslint

jslint:
	$(JS_LINT_COMMAND)

golint: scripts/format/format
	cd -P . && govendor vet +local
	# Run golint
	EXIT_CODE=0; \
	for package in $(PACKAGES) ; do \
		if [[ $$package != *minion/pb* && $$package != *api/pb* ]] ; then \
			golint -min_confidence .25 -set_exit_status $$package || EXIT_CODE=1; \
		fi \
	done ; \
	find . \( -path ./vendor -or -path ./node_modules -or -path ./docs/build \) -prune -or -name '*' -type f -print | xargs misspell -error || EXIT_CODE=1; \
	ineffassign . || EXIT_CODE=1; \
	exit $$EXIT_CODE
	# Run gofmt
	RESULT=`gofmt -s -l $(NOVENDOR)` && \
	if [[ -n "$$RESULT"  ]] ; then \
	    echo $$RESULT && \
	    exit 1 ; \
	fi
	# Do some additional checks of the go code (e.g., for line length)
	scripts/format/format $(filter-out $(LINE_LENGTH_EXCLUDE),$(NOVENDOR))

generate:
	govendor generate +local

providers:
	python3 scripts/gce-descriptions > cluster/machine/google.go

# This is what's strictly required for `make check lint` to run.
get-build-tools:
	go get -v -u \
	    github.com/client9/misspell/cmd/misspell \
	    github.com/golang/lint/golint \
	    github.com/gordonklaus/ineffassign \
	    github.com/kardianos/govendor
	npm install .

# This additionally contains the tools needed for `go generate` to work.
go-get: get-build-tools
	go get -v -u \
	    github.com/golang/protobuf/{proto,protoc-gen-go} \
	    github.com/vektra/mockery/.../

docker-build-quilt: linux
	cd -P . && git show --pretty=medium --no-patch > buildinfo \
	    && ${DOCKER} build -t ${REPO}/quilt .

docker-push-quilt:
	${DOCKER} push ${REPO}/quilt

docker-build-ovs:
	cd -P ovs && docker build -t ${REPO}/ovs .

# Include all .mk files so you can have your own local configurations
include $(wildcard *.mk)
