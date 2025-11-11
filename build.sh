#!/bin/sh

# export GOOS=linux
# export GOARCH=amd64

if [ -d ./dist ]; then
    rm -rf ./dist
fi
mkdir -p ./dist

cd ./src
go mod tidy
go get
cd ..
go build -C ./src -o ../dist/dn42-wiki-go -ldflags="-X main.GIT_COMMIT=$(git rev-parse --short HEAD)"
if [ $? -ne 0 ]; then
    echo "Build failed"
    exit 1
fi

cp -r ./template ./dist/
cp ./config.example.json ./dist/config.json

echo "Build succeeded. Artifact in ./dist"
exit 0
