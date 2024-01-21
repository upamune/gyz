.PHONY: build
build: clean
	@go build -o dist/gyz

.PHONY: clean
clean:
	@rm -fr dist
