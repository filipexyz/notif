use anyhow::Result;
use rusqlite::params;
use serde::{Deserialize, Serialize};

use crate::db::get_connection;

/// Sync state for a remote server
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SyncState {
    pub remote_name: String,
    pub last_synced_id: i64,
    pub last_synced_at: Option<String>,
}

/// Initialize the sync state table
pub fn init_sync_table() -> Result<()> {
    let conn = get_connection()?;

    conn.execute(
        "CREATE TABLE IF NOT EXISTS remote_sync_state (
            remote_name TEXT PRIMARY KEY,
            last_synced_id INTEGER NOT NULL DEFAULT 0,
            last_synced_at TEXT
        )",
        [],
    )?;

    Ok(())
}

/// Get sync state for a remote
pub fn get_sync_state(remote_name: &str) -> Result<SyncState> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT remote_name, last_synced_id, last_synced_at
         FROM remote_sync_state
         WHERE remote_name = ?1",
    )?;

    let result = stmt.query_row([remote_name], |row| {
        Ok(SyncState {
            remote_name: row.get(0)?,
            last_synced_id: row.get(1)?,
            last_synced_at: row.get(2)?,
        })
    });

    match result {
        Ok(state) => Ok(state),
        Err(rusqlite::Error::QueryReturnedNoRows) => Ok(SyncState {
            remote_name: remote_name.to_string(),
            last_synced_id: 0,
            last_synced_at: None,
        }),
        Err(e) => Err(e.into()),
    }
}

/// Update sync state for a remote
pub fn update_sync_state(remote_name: &str, last_synced_id: i64) -> Result<()> {
    let conn = get_connection()?;
    let now = chrono::Utc::now().to_rfc3339();

    conn.execute(
        "INSERT INTO remote_sync_state (remote_name, last_synced_id, last_synced_at)
         VALUES (?1, ?2, ?3)
         ON CONFLICT(remote_name) DO UPDATE SET
            last_synced_id = excluded.last_synced_id,
            last_synced_at = excluded.last_synced_at",
        params![remote_name, last_synced_id, now],
    )?;

    Ok(())
}

/// Get all sync states
pub fn get_all_sync_states() -> Result<Vec<SyncState>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT remote_name, last_synced_id, last_synced_at FROM remote_sync_state",
    )?;

    let states = stmt.query_map([], |row| {
        Ok(SyncState {
            remote_name: row.get(0)?,
            last_synced_id: row.get(1)?,
            last_synced_at: row.get(2)?,
        })
    })?;

    Ok(states.filter_map(|r| r.ok()).collect())
}

/// Delete sync state for a remote
pub fn delete_sync_state(remote_name: &str) -> Result<()> {
    let conn = get_connection()?;
    conn.execute(
        "DELETE FROM remote_sync_state WHERE remote_name = ?1",
        [remote_name],
    )?;
    Ok(())
}
