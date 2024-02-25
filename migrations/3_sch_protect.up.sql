PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS schedule_protect (
    id INTEGER PRIMARY KEY,
    segment_id INTEGER NOT NULL UNIQUE,
    CONSTRAINT fk_segment_id FOREIGN KEY (segment_id) REFERENCES segment (id) ON DELETE CASCADE
);