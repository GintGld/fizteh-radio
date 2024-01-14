#!/bin/bash

# script that docker use as entrypoint

# prepare database
mkdir storage && \
    ./migrator \
    -storage-path=./storage/storage.sqlite \
    -migrations-path=migrations \
    -migrations-table=migrations

# entrypoint
./radio