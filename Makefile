all: lint build unit

OUTPUT_DIR="build/_output"

.PHONY: build
build:
	build/build.sh ${OUTPUT_DIR}

.PHONY: lint
lint:
	hack/lint-gofmt.sh
	hack/lint-generate-crds.sh

.PHONY: unit
unit:
	hack/unit.sh

.PHONY: run-ci-e2e-test
run-ci-e2e-test:
	hack/run-ci-e2e-test.sh

.PHONY: clean
clean:
	rm -rf ${OUTPUT_DIR}

.PHONY: local-run
local-run:
	hack/run-local.sh -a run

.PHONY: local-clean
local-clean:
	hack/run-local.sh -a cleanup

.PHONY: local-run-debug
local-run-debug:
	hack/run-local.sh -a run -d
