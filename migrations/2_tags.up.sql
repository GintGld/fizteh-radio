PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS tagType (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS tag (
    id   INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    type_id INTEGER NOT NULL,
    CONSTRAINT fk_type_id FOREIGN KEY (type_id) REFERENCES tagType (id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS libraryTag (
    id       INTEGER PRIMARY KEY,
    media_id INTEGER NOT NULL,
    tag_id   INTEGER NOT NULL,
    CONSTRAINT fk_library_id FOREIGN KEY (media_id) REFERENCES library (id) ON DELETE CASCADE,
    CONSTRAINT fk_tag_id     FOREIGN KEY (tag_id)   REFERENCES tag (id)     ON DELETE CASCADE
);

INSERT INTO tagType (name) VALUES
("format"), ("genre"), ("playlist"), ("mood"), ("language");