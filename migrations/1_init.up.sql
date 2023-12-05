CREATE TABLE IF NOT EXISTS editors
(
    id          INTEGER PRIMARY KEY,
    login       TEXT NOT NULL UNIQUE,
    pass_hash   BLOB NOT NULL
);

CREATE TABLE IF NOT EXISTS library
(
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    author      TEXT NOT NULL,
    duration    INTEGER
);

CREATE TABLE IF NOT EXISTS schedule
(
    id          INTEGER PRIMARY KEY
    media_id    INTEGER
    period      INTEGER UNIQUE
    utc_time    INTEGER
    begin       INTEGER
    end         INTEGER
);

CREATE INDEX IF NOT EXISTS idx_period ON schedule (period);