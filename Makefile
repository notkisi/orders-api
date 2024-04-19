include .envrc
# ==================================================================================== #
# HELPERS
# ==================================================================================== #

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build_api: builds the api binary as a linux executable
.PHONY: build_api
build_api:
	@echo "Building api binary"
	env GOOS=linux CGO_ENABLED=0 go build -o bin/${API_BINARY} .
	@echo "Done!"
