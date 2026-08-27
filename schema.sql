CREATE TABLE IF NOT EXISTS drugs (
    id              TEXT PRIMARY KEY,
    brand_name      TEXT NOT NULL DEFAULT '',
    generic_name    TEXT NOT NULL DEFAULT '',
    manufacturer    TEXT NOT NULL DEFAULT '',
    product_ndc     TEXT NOT NULL DEFAULT '',
    product_type    TEXT NOT NULL DEFAULT '',
    route           TEXT NOT NULL DEFAULT '',
    substance_name  TEXT NOT NULL DEFAULT '',
    synced_at       TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_drugs_brand_name ON drugs (LOWER(brand_name));