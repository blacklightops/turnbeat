#!/bin/bash -x
CGO_ENABLED=0 GOOS=linux go build -a -tags netgo -ldflags '-w' . && docker build --tag=perspicaio/turnbeat:latest .
