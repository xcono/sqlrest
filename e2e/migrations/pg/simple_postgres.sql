-- Simple Test Dataset for E2E Testing (PostgreSQL)
-- This dataset is designed to be simple and reliable for testing

-- Drop existing tables if they exist
DROP TABLE IF EXISTS playlist_track CASCADE;
DROP TABLE IF EXISTS track CASCADE;
DROP TABLE IF EXISTS album CASCADE;
DROP TABLE IF EXISTS artist CASCADE;
DROP TABLE IF EXISTS genre CASCADE;

-- Create tables
CREATE TABLE artist (
    artist_id INT NOT NULL GENERATED ALWAYS AS IDENTITY,
    name VARCHAR(120) NOT NULL,
    PRIMARY KEY (artist_id)
);

CREATE TABLE genre (
    genre_id INT NOT NULL GENERATED ALWAYS AS IDENTITY,
    name VARCHAR(120) NOT NULL,
    PRIMARY KEY (genre_id)
);

CREATE TABLE album (
    album_id INT NOT NULL GENERATED ALWAYS AS IDENTITY,
    title VARCHAR(160) NOT NULL,
    artist_id INT NOT NULL,
    PRIMARY KEY (album_id),
    FOREIGN KEY (artist_id) REFERENCES artist(artist_id)
);

CREATE TABLE track (
    track_id INT NOT NULL GENERATED ALWAYS AS IDENTITY,
    name VARCHAR(200) NOT NULL,
    album_id INT,
    genre_id INT,
    milliseconds INT,
    unit_price DECIMAL(10,2),
    PRIMARY KEY (track_id),
    FOREIGN KEY (album_id) REFERENCES album(album_id),
    FOREIGN KEY (genre_id) REFERENCES genre(genre_id)
);

CREATE TABLE playlist_track (
    playlist_id INT NOT NULL,
    track_id INT NOT NULL,
    PRIMARY KEY (playlist_id, track_id),
    FOREIGN KEY (track_id) REFERENCES track(track_id)
);

-- Insert test data
INSERT INTO artist (name) VALUES 
('AC/DC'),
('Accept'),
('Aerosmith'),
('Alanis Morissette'),
('Alice In Chains');

INSERT INTO genre (name) VALUES 
('Rock'),
('Jazz'),
('Metal'),
('Alternative'),
('Pop');

INSERT INTO album (title, artist_id) VALUES 
('For Those About To Rock We Salute You', 1),
('Balls to the Wall', 2),
('Restless and Wild', 2),
('Let There Be Rock', 1),
('Big Ones', 3);

INSERT INTO track (name, album_id, genre_id, milliseconds, unit_price) VALUES 
('For Those About To Rock (We Salute You)', 1, 1, 343719, 0.99),
('Put The Finger On You', 1, 1, 205662, 0.99),
('Lets Get It Up', 1, 1, 217034, 0.99),
('Inject The Venom', 1, 1, 210834, 0.99),
('Snowballed', 1, 1, 203102, 0.99),
('Evil Walks', 1, 1, 263497, 0.99),
('C.O.D.', 1, 1, 199836, 0.99),
('Breaking The Rules', 1, 1, 263288, 0.99),
('Night Of The Long Knives', 1, 1, 205688, 0.99),
('Spellbound', 1, 1, 344853, 0.99);

INSERT INTO playlist_track (playlist_id, track_id) VALUES 
(1, 1),
(1, 2),
(1, 3),
(2, 4),
(2, 5),
(3, 6),
(3, 7),
(4, 8),
(4, 9),
(5, 10);
