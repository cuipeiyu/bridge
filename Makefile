export GOARCH=amd64
export GOOS=linux

# build
build:
	go build -o dist/bridge
	cp bridge.conf bridge.service dist
