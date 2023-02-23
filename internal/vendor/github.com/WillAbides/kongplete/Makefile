GOCMD=go
GOBUILD=$(GOCMD) build

bin/golangci-lint:
	script/bindown install $(notdir $@)

bin/goreadme:
	GOBIN=${CURDIR}/bin \
	go install github.com/posener/goreadme/cmd/goreadme@v1.4.2

.PHONY: clean
clean:
	rm -rf ./bin

bin/shellcheck:
	script/bindown install $(notdir $@)

bin/gofumpt:
	script/bindown install $(notdir $@)

HANDCRAFTED_REV := 082e94edadf89c33db0afb48889c8419a2cb46a9
bin/handcrafted:
	GOBIN=${CURDIR}/bin \
	go install github.com/willabides/handcrafted@$(HANDCRAFTED_REV)
