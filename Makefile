PROJECT_NAME := Pulumi pve Resource Provider

PACK             := pve
PACKDIR          := sdk
PROJECT          := github.com/hctamu/pulumi-pve

PROVIDER        := pulumi-resource-${PACK}
VERSION         ?= $(shell pulumictl get version)
PROVIDER_PATH   := provider
VERSION_PATH    := ${PROVIDER_PATH}.Version

GOPATH		:= $(shell go env GOPATH)

WORKING_DIR     := $(shell pwd)
EXAMPLES_DIR    := ${WORKING_DIR}/examples/yaml
TESTPARALLELISM := 4

OS    := $(shell uname)
SHELL := /bin/bash

GOFLAGS ?=

prepare::
	@if test -z "${NAME}"; then echo "NAME not set"; exit 1; fi
	@if test -z "${REPOSITORY}"; then echo "REPOSITORY not set"; exit 1; fi
	@if test -z "${ORG}"; then echo "ORG not set"; exit 1; fi
	@if test ! -d "provider/cmd/pulumi-resource-xyz"; then "Project already prepared"; exit 1; fi # SED_SKIP

	mv "provider/cmd/pulumi-resource-xyz" provider/cmd/pulumi-resource-${NAME} # SED_SKIP

	if [[ "${OS}" != "Darwin" ]]; then \
		find . \( -path './.git' -o -path './sdk' \) -prune -o -not -name 'go.sum' -type f -exec sed -i '/SED_SKIP/!s,github.com/pulumi/pulumi-[x]yz,${REPOSITORY},g' {} \; &> /dev/null; \
		find . \( -path './.git' -o -path './sdk' \) -prune -o -not -name 'go.sum' -type f -exec sed -i '/SED_SKIP/!s/[xX]yz/${NAME}/g' {} \; &> /dev/null; \
		find . \( -path './.git' -o -path './sdk' \) -prune -o -not -name 'go.sum' -type f -exec sed -i '/SED_SKIP/!s/[aA]bc/${ORG}/g' {} \; &> /dev/null; \
	fi

	# In MacOS the -i parameter needs an empty string to execute in place.
	if [[ "${OS}" == "Darwin" ]]; then \
		find . \( -path './.git' -o -path './sdk' \) -prune -o -not -name 'go.sum' -type f -exec sed -i '' '/SED_SKIP/!s,github.com/pulumi/pulumi-[x]yz,${REPOSITORY},g' {} \; &> /dev/null; \
		find . \( -path './.git' -o -path './sdk' \) -prune -o -not -name 'go.sum' -type f -exec sed -i '' '/SED_SKIP/!s/[xX]yz/${NAME}/g' {} \; &> /dev/null; \
		find . \( -path './.git' -o -path './sdk' \) -prune -o -not -name 'go.sum' -type f -exec sed -i '' '/SED_SKIP/!s/[aA]bc/${ORG}/g' {} \; &> /dev/null; \
	fi

ensure:
	cd provider && go mod tidy
	cd sdk && go mod tidy
	cd tests && go mod tidy

provider: $(WORKING_DIR)/bin/$(PROVIDER)
$(WORKING_DIR)/bin/$(PROVIDER): $(shell find . -name "*.go")
	go build $(GOFLAGS) -o $(WORKING_DIR)/bin/${PROVIDER} -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION}" $(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER)

provider_debug:
	(cd provider && go build -o $(WORKING_DIR)/bin/${PROVIDER} -gcflags="all=-N -l" -ldflags "-X ${PROJECT}/${VERSION_PATH}=${VERSION}" $(PROJECT)/${PROVIDER_PATH}/cmd/$(PROVIDER))

test_provider:
	cd tests && go test -short -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM} ./...

go_sdk: $(WORKING_DIR)/bin/$(PROVIDER)
	rm -rf sdk/go
	pulumi package gen-sdk $(WORKING_DIR)/bin/$(PROVIDER) --language go

gen_examples: gen_go_example

gen_%_example:
	rm -rf ${WORKING_DIR}/examples/$*
	pulumi convert \
		--cwd ${WORKING_DIR}/examples/yaml \
		--logtostderr \
		--generate-only \
		--non-interactive \
		--language $* \
		--out ${WORKING_DIR}/examples/$*

define pulumi_login
	if [ -z "$$PULUMI_CONFIG_PASSPHRASE" ]; then \
		echo "Error: PULUMI_CONFIG_PASSPHRASE is not set"; \
		exit 1; \
	fi; \
	pulumi login --local;
endef

stack_init::
	$(call pulumi_login) \
	cd ${EXAMPLES_DIR} && \
	pulumi stack init dev && \
	pulumi stack select dev && \
	pulumi config set name dev

up:: stack_init
	pulumi up -y

update::
	$(call pulumi_login) \
	cd ${EXAMPLES_DIR} && \
	pulumi stack select dev && \
	pulumi config set name dev && \
	pulumi up -y

refresh::
	$(call pulumi_login) \
	cd ${EXAMPLES_DIR} && \
	pulumi stack select dev && \
	pulumi config set name dev && \
	pulumi refresh -y

preview::
	$(call pulumi_login) \
	cd ${EXAMPLES_DIR} && \
	pulumi stack select dev && \
	pulumi config set name dev && \
	pulumi preview --diff --color always

down::
	$(call pulumi_login) \
	cd ${EXAMPLES_DIR} && \
	pulumi stack select dev && \
	pulumi destroy -y && \
	pulumi stack rm dev -y --preserve-config

.PHONY: build
build: provider go_sdk

# Required for the codegen action that runs in pulumi/pulumi
only_build: build

lint:
	for DIR in "provider" "sdk" "tests" ; do \
		pushd $$DIR && golangci-lint run -c ../.golangci.yml --timeout 10m && popd ; \
	done

install:
	mkdir -p ${GOPATH}/bin && \
	cp $(WORKING_DIR)/bin/${PROVIDER} ${GOPATH}/bin/${PROVIDER}

GO_TEST	 := go test -v -count=1 -cover -timeout 2h -parallel ${TESTPARALLELISM}

test_all: test_provider
	cd tests/sdk/go && $(GO_TEST) ./...

install_go_sdk:
	#target intentionally blank

.PHONY: release
release:
	@read -p "Enter version (x.x.x): " version; \
	read -p "Enter commit message: " commit_msg; \
	read -p "Enter tag message: " tag_msg; \
	echo "Adding all changes..."; \
	git add .; \
	echo "Committing with message: feat: $$commit_msg"; \
	git commit -m "feat: $$commit_msg"; \
	echo "Creating tag $$version with message: $$tag_msg"; \
    git tag -a v$$version -m "$$tag_msg"; \
    git tag -a sdk/v$$version -m "$$tag_msg"; \
	echo "Pushing commit and tags..."; \
	git push --follow-tags