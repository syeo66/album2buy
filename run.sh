#!/bin/sh
set -e
tmpFile=$(mktemp)
go build -o "$tmpFile" *.go
dotenvx run -- $tmpFile

