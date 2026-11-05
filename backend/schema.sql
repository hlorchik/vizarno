CREATE TABLE IF NOT EXISTS sellers (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email text NOT NULL UNIQUE, salt bytea NOT NULL, password_hash bytea NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
  digest bytea PRIMARY KEY, seller_id bigint NOT NULL REFERENCES sellers(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS cards (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  seller_id bigint NOT NULL REFERENCES sellers(id) ON DELETE CASCADE, title text NOT NULL
);
CREATE INDEX IF NOT EXISTS cards_owner ON cards (seller_id,id);
CREATE TABLE IF NOT EXISTS versions (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, card_id bigint NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now(), mime text NOT NULL,
  cover bytea NOT NULL, report jsonb NOT NULL
);
CREATE INDEX IF NOT EXISTS versions_card ON versions (card_id,id);


-- Upgrade foreign keys created before account deletion was supported.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='cards'::regclass AND conname='cards_seller_id_fkey' AND confdeltype <> 'c') THEN
    ALTER TABLE cards DROP CONSTRAINT cards_seller_id_fkey,
      ADD CONSTRAINT cards_seller_id_fkey FOREIGN KEY (seller_id) REFERENCES sellers(id) ON DELETE CASCADE;
  END IF;
  IF EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid='versions'::regclass AND conname='versions_card_id_fkey' AND confdeltype <> 'c') THEN
    ALTER TABLE versions DROP CONSTRAINT versions_card_id_fkey,
      ADD CONSTRAINT versions_card_id_fkey FOREIGN KEY (card_id) REFERENCES cards(id) ON DELETE CASCADE;
  END IF;
END $$;

ALTER TABLE cards ADD COLUMN IF NOT EXISTS profile jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE versions ADD COLUMN IF NOT EXISTS ai_result jsonb;
CREATE TABLE IF NOT EXISTS gallery (
  version_id bigint NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
  slide_no integer NOT NULL CHECK (slide_no BETWEEN 2 AND 5),
  mime text NOT NULL, image bytea NOT NULL, PRIMARY KEY(version_id,slide_no)
);

CREATE TABLE IF NOT EXISTS generated_sets (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 card_id bigint NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
 version_id bigint NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
 profile jsonb NOT NULL, pages integer NOT NULL CHECK(pages BETWEEN 1 AND 8),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS generated_card ON generated_sets(card_id,id);
CREATE TABLE IF NOT EXISTS generated_pages (
 set_id bigint NOT NULL REFERENCES generated_sets(id) ON DELETE CASCADE,
 page integer NOT NULL CHECK(page BETWEEN 1 AND 8), png bytea NOT NULL,
 PRIMARY KEY(set_id,page)
);

ALTER TABLE generated_sets ADD COLUMN IF NOT EXISTS method_report jsonb;
-- Older installations allowed only six pages. The current release plan contains eight.
ALTER TABLE generated_sets DROP CONSTRAINT IF EXISTS generated_sets_pages_check;
ALTER TABLE generated_sets ADD CONSTRAINT generated_sets_pages_check CHECK(pages BETWEEN 1 AND 8);
ALTER TABLE generated_pages DROP CONSTRAINT IF EXISTS generated_pages_page_check;
ALTER TABLE generated_pages ADD CONSTRAINT generated_pages_page_check CHECK(page BETWEEN 1 AND 8);
CREATE TABLE IF NOT EXISTS method_approvals (
 version_id bigint NOT NULL REFERENCES versions(id) ON DELETE CASCADE,
 digest bytea NOT NULL, report jsonb NOT NULL,
 PRIMARY KEY(version_id,digest)
);
