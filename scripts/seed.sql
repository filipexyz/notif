-- Seed data for development
-- Run this after migrations: psql $DATABASE_URL -f scripts/seed.sql

-- Create default project for dev org
INSERT INTO projects (id, org_id, name, slug, created_at, updated_at)
VALUES (
    'prj_dev_default_000000000000000',
    'org_dev_test',
    'Default',
    'default',
    NOW(),
    NOW()
) ON CONFLICT (org_id, slug) DO NOTHING;

-- Test key: nsh_testkey1234567890abcdefghijk (32 chars: nsh_ + 28 alphanumeric)
-- Linked to the dev default project
INSERT INTO api_keys (key_hash, key_prefix, name, org_id, project_id)
VALUES (
    '49fa6482e6112e24939cd4c42aab7b904c7e33da2ddc6e8dcf60686055859b1e',
    'nsh_testkey12345',
    'Dev Test Key',
    'org_dev_test',
    'prj_dev_default_000000000000000'
) ON CONFLICT (key_hash) DO UPDATE SET project_id = EXCLUDED.project_id;
