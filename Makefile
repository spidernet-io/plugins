include Makefile.defs

all: usage

usage:
	@echo "usage:"
	@echo  "  \033[35m make build \033[0m:       --- build all plugins"
	@echo  "  \033[35m make test \033[0m:        --- run e2e test on your local environment"

.PHONY: build
build:
	@mkdir -p ./.tmp/bin ; \
	for plugin in `ls ./plugins/` ; do   \
		echo "\033[35m ==> building $${plugin} to $(ROOT_DIR)/.tmp/bin/${plugin}  \033[0m" ; \
		$(GO_BUILD_FLAGS) $(GO_BUILD) $(GO_BUILD_LDFLGAS) -o ./.tmp/bin/$${plugin} ./plugins/$${plugin} ;  \
	done

.PHONY: lint-golang
lint-golang:
	GOOS=linux golangci-lint run ./...

.PHONY: unit-test
unit-test:
	 ginkgo version ; \
	 ginkgo --cover --coverprofile=coverage.out --covermode set \
	 	--json-report unitestreport.json --label-filter="${LABELS}" -randomize-suites -randomize-all --keep-going \
  	 	--timeout=1h   --slow-spec-threshold=120s -vv  -gcflags=-l -r pkg/* plugins/*
	go tool cover -html=./coverage.out -o coverage-all.html