-- +goose Up

-- The list is read one page at a time, ordered by (created_at, id) and continued from a
-- cursor on that same pair. With only idx_document_records_tenant to work with, every
-- page sorted the tenant's whole table to return two hundred rows.
--
-- Measured on a tenant holding 20,007 documents:
--
--   without this index   2.5 ms   Seq Scan + top-N sort of 20,007 rows
--   with it              0.06 ms  Index Only Scan reading 200 entries
--
-- The gap grows with the tenant, because the unindexed plan reads everything the tenant
-- has whatever the page size is, and the indexed one reads the page.
--
-- One index, not two. The approval queue also filters on status, and an index leading
-- with status serves that 0.08 ms faster — but it cannot serve the unfiltered list at
-- all, which falls back to the sort. This one serves both: the queue's page comes out at
-- 0.14 ms, filtering the few rows of other statuses out of the range it walks. A second
-- index would buy 0.08 ms on one screen and cost every write.
--
-- Postgres reads a b-tree backwards as happily as forwards, so this covers newest-first
-- and oldest-first alike.
CREATE INDEX IF NOT EXISTS idx_document_records_page
    ON document_records (tenant_id, created_at, id);

-- +goose Down
DROP INDEX IF EXISTS idx_document_records_page;
