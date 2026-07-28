.PHONY: test cover cover-html build vet clean rice-box

## build compiles the nitr binary into the repo root
build:
	go build -o nitr

## rice-box regenerates the committed embedded-asset box (rice-box.go) from
## the files under app/assets and app/views. Run this whenever an asset
## changes, then commit rice-box.go alongside the source change. The box is
## the only copy the binary ships, so a stale box silently serves an old UI.
## Requires the rice tool: go install github.com/GeertJohan/rice@latest
rice-box:
	rice embed-go

## test runs the full test suite
test:
	go test ./...

## cover prints per-function coverage percentages
cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

## cover-html generates an HTML coverage report and opens it
cover-html:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out

## vet runs go vet on every package
vet:
	go vet ./...

## clean removes generated artefacts and test output
clean:
	rm -f coverage.out coverage.html nitr
