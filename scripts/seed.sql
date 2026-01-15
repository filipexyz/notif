-- Seed API keys for development
-- Run this after migrations: psql $DATABASE_URL -f scripts/seed.sql

-- Test key: nsh_testkey1234567890abcdefghijk (32 chars: nsh_ + 28 alphanumeric)
INSERT INTO api_keys (key_hash, key_prefix, name, org_id)
VALUES (
    '49fa6482e6112e24939cd4c42aab7b904c7e33da2ddc6e8dcf60686055859b1e',
    'nsh_testkey12345',
    'Dev Test Key',
    'org_dev_test'
) ON CONFLICT (key_hash) DO NOTHING;
