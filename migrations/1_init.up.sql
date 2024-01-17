CREATE TABLE IF NOT EXISTS editors
(
    id          INTEGER PRIMARY KEY,
    login       TEXT NOT NULL UNIQUE,
    pass_hash   BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_login ON editors (login);

CREATE TABLE IF NOT EXISTS library
(
    id          INTEGER PRIMARY KEY,
    name        TEXT NOT NULL,
    author      TEXT NOT NULL,
    duration    INTEGER,
    source_id   INTEGER UNIQUE
);

CREATE TABLE IF NOT EXISTS schedule
(
    id          INTEGER PRIMARY KEY,
    media_id    INTEGER,
    start_mus    INTEGER,
    begin_cut   INTEGER,
    stop_cut    INTEGER
);