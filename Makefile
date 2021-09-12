.PHONY: build update release vendor version publish

build:
	go mod vendor
	go build -mod=vendor

update:
	go get -u

release:
	go build -ldflags="-s -w" -mod=vendor

vendor:
	go mod vendor

version:
	git tag "v$(VERSION)"
	jq -n --arg version $(VERSION) '{"version":$$version}' > docs/version.json

publish:
	goreleaser --rm-dist --skip-validate
