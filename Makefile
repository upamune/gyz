.PHONY: build
build: clean
	@go build -o dist/gyz

.PHONY: watch
watch:
	@air

.PHONY: clean
clean:
	@rm -fr dist
