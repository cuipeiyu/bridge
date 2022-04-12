release:
	GOOS=linux GOARCH=amd64 make build-unix
	GOOS=linux GOARCH=arm64 make build-unix

	GOOS=darwin GOARCH=amd64 make build-unix
	GOOS=darwin GOARCH=arm64 make build-unix

	GOOS=windows GOARCH=amd64 make build-windows

# build unix binrary
build-unix:
	mkdir -p dist/$(GOOS)-$(GOARCH)
	cp README.md bridge.conf bridge.service dist/$(GOOS)-$(GOARCH)
	go build -o dist/$(GOOS)-$(GOARCH)/bridge
	cd dist/$(GOOS)-$(GOARCH) && tar -zvcf ../bridge-$(GOOS)-$(GOARCH).tar.gz README.md bridge bridge.conf bridge.service

# build windows binrary
build-windows:
	mkdir -p dist/$(GOOS)-$(GOARCH)
	cp README.md bridge.conf bridge.service dist/$(GOOS)-$(GOARCH)
	go build -o dist/$(GOOS)-$(GOARCH)/bridge.exe
	cd dist/$(GOOS)-$(GOARCH) && tar -zvcf ../bridge-$(GOOS)-$(GOARCH).tar.gz README.md bridge.exe bridge.conf bridge.service

# show help
help:
	@echo ''
	@echo 'Usage:'
	@echo ' make [target]'
	@echo ''
	@echo 'Targets:'
	@awk '/^[a-zA-Z\-\_0-9]+:/ { \
	helpMessage = match(lastLine, /^# (.*)/); \
		if (helpMessage) { \
			helpCommand = substr($$1, 0, index($$1, ":")-1); \
			helpMessage = substr(lastLine, RSTART + 2, RLENGTH); \
			printf "\033[36m%-22s\033[0m %s\n", helpCommand,helpMessage; \
		} \
	} \
	{ lastLine = $$0 }' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
