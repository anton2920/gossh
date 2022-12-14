#!/bin/sh

GOARCH=386 GO386=softfloat GOOS=plan9 go build -v -ldflags '-s -w'
