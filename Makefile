LDFLAGS = -ldflags "-w -s"
GCTPKG = github.com/yurulab/gocryptotrader
LINTPKG = github.com/golangci/golangci-lint/cmd/golangci-lint@v1.24.0
LINTBIN = $(GOPATH)/bin/golangci-lint
GCTLISTENPORT=9050
GCTPROFILERLISTENPORT=8085
CRON = $(TRAVIS_EVENT_TYPE)
DRIVER ?= psql
RACE_FLAG := $(if $(NO_RACE_TEST),,-race)

get:
	GO111MODULE=on go get $(GCTPKG)

linter:
	GO111MODULE=on go get $(GCTPKG)
	GO111MODULE=on go get $(LINTPKG)
	test -z "$$($(LINTBIN) run --verbose | tee /dev/stderr)"

check: linter test

test:
ifeq ($(CRON), cron)
	go test $(RACE_FLAG) -tags=mock_test_off -coverprofile=coverage.txt -covermode=atomic  ./...
else
	go test $(RACE_FLAG) -coverprofile=coverage.txt -covermode=atomic  ./...
endif

build:
	GO111MODULE=on go build $(LDFLAGS)

install:
	GO111MODULE=on go install $(LDFLAGS)

fmt:
	gofmt -l -w -s $(shell find . -type f -name '*.go')

update_deps:
	GO111MODULE=on go mod verify
	GO111MODULE=on go mod tidy
	rm -rf vendor
	GO111MODULE=on go mod vendor

.PHONY: profile_heap
profile_heap:
	go tool pprof -http "localhost:$(GCTPROFILERLISTENPORT)" 'http://localhost:$(GCTLISTENPORT)/debug/pprof/heap'

.PHONY: profile_cpu
profile_cpu:
	go tool pprof -http "localhost:$(GCTPROFILERLISTENPORT)" 'http://localhost:$(GCTLISTENPORT)/debug/pprof/profile'

gen_db_models:
ifeq ($(DRIVER), psql)
	sqlboiler -o database/models/postgres -p postgres --no-auto-timestamps --wipe $(DRIVER)
else
	sqlboiler -o database/models/sqlite3 -p sqlite3 --no-auto-timestamps --wipe $(DRIVER)
endif
