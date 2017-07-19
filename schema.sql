CREATE TABLE review (
  id SERIAL PRIMARY KEY,
  store VARCHAR(255) NOT NULL,
  author VARCHAR(255) NULL,
  comment_uri VARCHAR(255) NULL,
  updated_at DATE NOT NULL
);
CREATE INDEX comment_uri_idx on review(comment_uri);
CREATE INDEX store_idx on review(store);
