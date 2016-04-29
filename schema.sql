CREATE TABLE review (
  id SERIAL PRIMARY KEY,
  author VARCHAR(255) NULL,
  author_uri VARCHAR(255) NULL,
  updated_at DATE NOT NULL
);
CREATE INDEX author_uri_idx on review(author_uri);
