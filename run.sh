#!/bin/bash

export SOURCE_STORAGE=/home/gintaras/workspace/radio/fizteh-radio/tmp
# export STATIC_FILES=/home/gintaras/workspace/radio/client
export STATIC_FILES=/home/gintaras/workspace/radio/fizteh-radio/public
export DB_SQLITE=/home/gintaras/workspace/radio/fizteh-radio/storage

# docker compose up
rm -rf storage
rm -rf ./tmp/*
mkdir -p ./tmp ./tmp/man ./tmp/content ./tmp/server

docker compose up