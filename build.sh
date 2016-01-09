#!/bin/sh

set -e

export GOOS=darwin
export GOARCH=amd64
go build -o projectcreator-mac -a -v -x
export GOOS=linux
go build -o projectcreator-linux
export GOOS=windows
go build -o projectcreator-win.exe

