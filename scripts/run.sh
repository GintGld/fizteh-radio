#!/bin/bash

# script that docker use as entrypoint

# prepare SQLite database
# migrator can't create dir
mkdir -p storage
./migrator \
    -storage-path=./storage/storage.sqlite \
    -migrations-path=migrations \
    -migrations-table=migrations

# entrypoint
./radio