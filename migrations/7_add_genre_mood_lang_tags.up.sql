-- Delete all genres, moods and languages to formalize these tag types.
DELETE FROM tag WHERE type_id IN (
    SELECT id FROM tagType WHERE name IN ('genre', 'mood', 'language')
);

-- Insert new tags.
INSERT INTO tag(name, type_id) VALUES
('Поп',                 (SELECT id from tagType WHERE name='genre')),
('Хих-хоп',             (SELECT id from tagType WHERE name='genre')),
('Рок',                 (SELECT id from tagType WHERE name='genre')),
('Джаз',                (SELECT id from tagType WHERE name='genre')),
('Электро',             (SELECT id from tagType WHERE name='genre')),
('Инструментальный',    (SELECT id from tagType WHERE name='genre')),
('Реп',                 (SELECT id from tagType WHERE name='genre')),
('Lo-fi',               (SELECT id from tagType WHERE name='genre'));

INSERT INTO tag(name, type_id) VALUES
('агрессивное',         (SELECT id from tagType WHERE name='mood')),
('оптимистичное',       (SELECT id from tagType WHERE name='mood')),
('спокойное',           (SELECT id from tagType WHERE name='mood')),
('тревожное',           (SELECT id from tagType WHERE name='mood')),
('ритмичное',           (SELECT id from tagType WHERE name='mood')),
('романтичное',         (SELECT id from tagType WHERE name='mood')),
('печальное',           (SELECT id from tagType WHERE name='mood'));

INSERT INTO tag(name, type_id) VALUES
('русский',             (SELECT id from tagType WHERE name='language')),
('английский',          (SELECT id from tagType WHERE name='language')),
('французский',         (SELECT id from tagType WHERE name='language')),
('итальянский',         (SELECT id from tagType WHERE name='language')),
('немецкий',            (SELECT id from tagType WHERE name='language')),
('испанский',           (SELECT id from tagType WHERE name='language')),
('монгольский',         (SELECT id from tagType WHERE name='language')),
('корейский',           (SELECT id from tagType WHERE name='language')),
('японский',            (SELECT id from tagType WHERE name='language')),
('китайский',           (SELECT id from tagType WHERE name='language')),
('без слов',            (SELECT id from tagType WHERE name='language'));
