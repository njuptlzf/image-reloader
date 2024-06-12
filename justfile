build:
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o _output/bin/image-reloader .

image: build
    buildah build -f quick.dockerfile .