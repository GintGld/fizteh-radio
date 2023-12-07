#!/bin/bash

source .env
# go run cmd/migrator/main.go \
#     --storage-path=/home/gintaras/workspace/server/storage/storage.sqlite \
#     --migrations-path=/home/gintaras/workspace/server/migrations
go run cmd/radio/main.go