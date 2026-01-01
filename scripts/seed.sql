-- Seed API keys for development
-- Run this after migrations: psql $DATABASE_URL -f scripts/seed.sql

-- Test key: nsh_test_abcdefghij12345678901234
INSERT INTO api_keys (key_hash, key_prefix, name, org_id)
VALUES (
    '96634cc1642dc070e95218752fabbf5bbf8410262de19ddbf9e3f1fa7e1e79b9',
    'nsh_test_abcdef',
    'Dev Test Key',
    'org_dev_test'
) ON CONFLICT (key_hash) DO NOTHING;
