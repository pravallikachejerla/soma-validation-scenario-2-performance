-- 0002_seed_meta.sql — seed control table used to record the
-- dataset hash and the timestamp of the last successful seed run.
CREATE TABLE IF NOT EXISTS seed_meta (
    id           INT PRIMARY KEY,
    dataset_sha  TEXT NOT NULL,
    profile      TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Backward-compatible note: there is no automatic data seed in this
-- migration. The seeder binary populates data via INSERT statements
-- generated from the synthetic dataset.
