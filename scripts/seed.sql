-- Seed API keys for development
-- Run this after migrations: psql $DATABASE_URL -f scripts/seed.sql

-- Test key: nsh_test_abcdefghij1234567890ab
INSERT INTO api_keys (key_hash, key_prefix, environment, name)
VALUES (
    '78fd02861ac7f3a7e0b9b78cb489c3e3f02f87462b6fe81253172118791d453c',
    'nsh_test_abcdef',
    'test',
    'Dev Test Key'
) ON CONFLICT (key_hash) DO NOTHING;
