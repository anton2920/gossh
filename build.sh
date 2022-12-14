#!/bin/sh

GOARCH=386 GO386=softfloat GOOS=plan9 go build -ldflags '-s -w'
