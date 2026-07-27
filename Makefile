.PHONY: test cover cover-html build vet clean

## build compiles the nitr binary and all packages
build:
	go build ./...

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
	rm -f coverage.out coverage.html
