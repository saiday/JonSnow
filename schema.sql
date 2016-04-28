CREATE TABLE review (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  author VARCHAR(255) NULL,
  author_uri VARCHAR(255) NULL,
  updated_at DATETIME NOT NULL
);
CREATE INDEX author_uri_idx on review(author_uri);
