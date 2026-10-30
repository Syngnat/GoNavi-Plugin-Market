-- Multiple reviews per user and one-level replies.
-- Existing rows become top-level reviews; stars stay 1..5 for reviews and
-- become 0 for replies so aggregate queries can exclude replies explicitly.
ALTER TABLE market_ratings ADD COLUMN id bigserial;
ALTER TABLE market_ratings DROP CONSTRAINT market_ratings_pkey;
ALTER TABLE market_ratings ADD CONSTRAINT market_ratings_pkey PRIMARY KEY (id);
ALTER TABLE market_ratings ADD COLUMN parent_id bigint REFERENCES market_ratings(id) ON DELETE CASCADE;
ALTER TABLE market_ratings ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE market_ratings DROP CONSTRAINT market_ratings_stars_check;
ALTER TABLE market_ratings ADD CONSTRAINT market_ratings_shape_check CHECK (
  (parent_id IS NULL AND stars BETWEEN 1 AND 5)
  OR (parent_id IS NOT NULL AND stars = 0)
);
CREATE INDEX market_ratings_replies ON market_ratings(parent_id, created_at);
