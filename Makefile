export GO15VENDOREXPERIMENT=1
PACKAGES=$(shell govendor list -no-status +local)
NOVENDOR=$(shell find . -path -prune -o -path ./vendor -prune -o -name '*.go' -print)
LINE_LENGTH_EXCLUDE=./api/pb/pb.pb.go \
		    ./cloud/amazon/client/mocks/% \
		    ./cloud/cfg/template.go \
		    ./cloud/digitalocean/client/mocks/% \
		    ./cloud/google/client/mocks/% \
		    ./cloud/machine/amazon.go \
		    ./cloud/machine/google.go \
		    ./minion/network/link_test.go \
		    ./minion/ovsdb/mock_transact_test.go \
		    ./minion/ovsdb/mocks/Client.go \
		    ./minion/pb/pb.pb.go \
		    ./node_modules/% \
		    ./quilt-tester/tests/zookeeper/vendor/%

JS_LINT_COMMAND = node_modules/eslint/bin/eslint.js js quilt-tester

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

jscheck:
	npm test

check: gocheck jscheck

clean:
	govendor clean -x +local
	rm -f *.cov.coverprofile cloud/*.cov.coverprofile minion/*.cov.coverprofile
	rm -f *.cov.html cloud/*.cov.html minion/*.cov.html
	rm quilt_linux quilt_darwin

linux:
	cd -P . && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o quilt_linux .

darwin:
	cd -P . && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o quilt_darwin .

release: linux darwin

COV_SKIP= /api/client/mocks \
	  /api/pb \
	  /cloud/amazon/client/mocks \
	  /cloud/digitalocean/client/mocks \
	  /cloud/google/client/mocks \
	  /cloud/provider/mocks \
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
	echo "" > coverage.txt
	for package in $(filter-out $(COV_SKIP), $(subst github.com/quilt/quilt,,$(PACKAGES))) ; do \
	    go test -coverprofile=.$$package.cov .$$package && \
	    go tool cover -html=.$$package.cov -o .$$package.html ; \
	    cat .$$package.cov >> coverage.txt ; \
	done

js-coverage:
	./node_modules/.bin/nyc npm test
	./node_modules/.bin/nyc report --reporter=text-lcov > coverage.lcov

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
	python3 scripts/gce-descriptions > cloud/machine/google.go

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

# Prepare the js/install directory for either `npm install` or `npm publish`.
prep-install: linux darwin js/install/package.json
	cp quilt_linux js/install
	cp quilt_darwin js/install
	cp -r js/initializer js/install

js/install/package.json: js/install/package_template.json
	node js/install/make_package.js > js/install/package.json
