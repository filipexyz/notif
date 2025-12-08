use anyhow::{Context, Result};
use directories::ProjectDirs;
use rusqlite::{params, Connection};
use std::fs;
use std::path::PathBuf;

use crate::models::{Notification, Priority, SourceType, Status, TagFilter};

pub fn get_db_path() -> Result<PathBuf> {
    let proj_dirs = ProjectDirs::from("com", "filipelabs", "notif")
        .context("Could not determine config directory")?;

    let data_dir = proj_dirs.data_dir();
    fs::create_dir_all(data_dir).context("Could not create data directory")?;

    Ok(data_dir.join("notif.db"))
}

pub fn get_connection() -> Result<Connection> {
    let db_path = get_db_path()?;
    let conn = Connection::open(&db_path)
        .with_context(|| format!("Could not open database at {:?}", db_path))?;
    Ok(conn)
}

pub fn init_db() -> Result<()> {
    let conn = get_connection()?;

    conn.execute(
        "CREATE TABLE IF NOT EXISTS notifications (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            message TEXT NOT NULL,
            priority TEXT NOT NULL DEFAULT 'normal',
            status TEXT NOT NULL DEFAULT 'pending',
            tags TEXT NOT NULL DEFAULT '',
            created_at TEXT NOT NULL,
            delivered_at TEXT,
            content TEXT
        )",
        [],
    )?;

    // Migration: add tags column if it doesn't exist (for existing DBs)
    let _ = conn.execute("ALTER TABLE notifications ADD COLUMN tags TEXT NOT NULL DEFAULT ''", []);
    // Migration: add content column if it doesn't exist
    let _ = conn.execute("ALTER TABLE notifications ADD COLUMN content TEXT", []);
    // Migration: add source columns for remote notification tracking
    let _ = conn.execute("ALTER TABLE notifications ADD COLUMN source_type TEXT NOT NULL DEFAULT 'local'", []);
    let _ = conn.execute("ALTER TABLE notifications ADD COLUMN source_remote TEXT", []);
    let _ = conn.execute("ALTER TABLE notifications ADD COLUMN source_id INTEGER", []);

    conn.execute(
        "CREATE INDEX IF NOT EXISTS idx_status ON notifications(status)",
        [],
    )?;

    conn.execute(
        "CREATE INDEX IF NOT EXISTS idx_priority ON notifications(priority)",
        [],
    )?;

    conn.execute(
        "CREATE INDEX IF NOT EXISTS idx_source ON notifications(source_type, source_remote, source_id)",
        [],
    )?;

    // Create sync state table
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

pub fn add_notification(message: &str, priority: Priority, tags: &[String], status: Status) -> Result<i64> {
    add_notification_with_content(message, priority, tags, status, None)
}

pub fn add_notification_with_content(
    message: &str,
    priority: Priority,
    tags: &[String],
    status: Status,
    content: Option<&str>,
) -> Result<i64> {
    let conn = get_connection()?;
    let now = chrono::Utc::now().to_rfc3339();
    let tags_str = tags.join(",");

    conn.execute(
        "INSERT INTO notifications (message, priority, status, tags, created_at, content) VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
        params![message, priority.to_string(), status.to_string(), tags_str, now, content],
    )?;

    Ok(conn.last_insert_rowid())
}

fn parse_tags(tags_str: &str) -> Vec<String> {
    if tags_str.is_empty() {
        Vec::new()
    } else {
        tags_str.split(',').map(|s| s.trim().to_string()).collect()
    }
}

fn row_to_notification(row: &rusqlite::Row) -> rusqlite::Result<Notification> {
    let priority_str: String = row.get(2)?;
    let priority = priority_str.parse().unwrap_or(Priority::Normal);
    let status_str: String = row.get(3)?;
    let status = status_str.parse().unwrap_or(Status::Pending);
    let tags_str: String = row.get(4)?;
    let source_type_str: String = row.get(8)?;
    let source_type = source_type_str.parse().unwrap_or(SourceType::Local);

    Ok(Notification {
        id: row.get(0)?,
        message: row.get(1)?,
        priority,
        status,
        tags: parse_tags(&tags_str),
        created_at: row.get(5)?,
        delivered_at: row.get(6)?,
        content: row.get(7)?,
        source_type,
        source_remote: row.get(9)?,
        source_id: row.get(10)?,
    })
}

fn get_by_status_filtered(status: &str, limit: usize, filter: Option<&TagFilter>) -> Result<Vec<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at, content, source_type, source_remote, source_id
         FROM notifications
         WHERE status = ?1
         ORDER BY
            CASE priority
                WHEN 'high' THEN 1
                WHEN 'normal' THEN 2
                WHEN 'low' THEN 3
            END,
            created_at ASC"
    )?;

    let notifications = stmt.query_map([status], row_to_notification)?;
    let all: Vec<Notification> = notifications.filter_map(|r| r.ok()).collect();

    // Apply tag filter in memory
    let filtered: Vec<Notification> = match filter {
        Some(f) => all.into_iter().filter(|n| f.matches(&n.tags)).collect(),
        None => all,
    };

    Ok(filtered.into_iter().take(limit).collect())
}

// Pending notifications (awaiting review)
pub fn get_pending_filtered(limit: usize, filter: Option<&TagFilter>) -> Result<Vec<Notification>> {
    get_by_status_filtered("pending", limit, filter)
}

pub fn get_pending(limit: usize) -> Result<Vec<Notification>> {
    get_pending_filtered(limit, None)
}

pub fn get_all_pending() -> Result<Vec<Notification>> {
    get_pending_filtered(1000, None)
}

pub fn get_all_pending_filtered(filter: Option<&TagFilter>) -> Result<Vec<Notification>> {
    get_pending_filtered(1000, filter)
}

// Approved notifications (ready for injection)
pub fn get_approved_filtered(limit: usize, filter: Option<&TagFilter>) -> Result<Vec<Notification>> {
    get_by_status_filtered("approved", limit, filter)
}

pub fn get_approved(limit: usize) -> Result<Vec<Notification>> {
    get_approved_filtered(limit, None)
}

pub fn get_all_approved() -> Result<Vec<Notification>> {
    get_approved_filtered(1000, None)
}

// Status updates
pub fn set_status(id: i64, status: Status) -> Result<()> {
    let conn = get_connection()?;
    let now = if status == Status::Delivered {
        Some(chrono::Utc::now().to_rfc3339())
    } else {
        None
    };

    conn.execute(
        "UPDATE notifications SET status = ?1, delivered_at = ?2 WHERE id = ?3",
        params![status.to_string(), now, id],
    )?;

    Ok(())
}

pub fn approve(id: i64) -> Result<()> {
    set_status(id, Status::Approved)
}

pub fn dismiss(id: i64) -> Result<()> {
    set_status(id, Status::Dismissed)
}

pub fn mark_delivered(ids: &[i64]) -> Result<()> {
    for id in ids {
        set_status(*id, Status::Delivered)?;
    }
    Ok(())
}

pub fn approve_all_pending() -> Result<usize> {
    let conn = get_connection()?;
    let updated = conn.execute(
        "UPDATE notifications SET status = 'approved' WHERE status = 'pending'",
        [],
    )?;
    Ok(updated)
}

pub fn dismiss_all_pending() -> Result<usize> {
    let conn = get_connection()?;
    let updated = conn.execute(
        "UPDATE notifications SET status = 'dismissed' WHERE status = 'pending'",
        [],
    )?;
    Ok(updated)
}

pub fn clear_delivered() -> Result<usize> {
    let conn = get_connection()?;
    let deleted = conn.execute("DELETE FROM notifications WHERE status = 'delivered'", [])?;
    Ok(deleted)
}

pub fn clear_dismissed() -> Result<usize> {
    let conn = get_connection()?;
    let deleted = conn.execute("DELETE FROM notifications WHERE status = 'dismissed'", [])?;
    Ok(deleted)
}

pub fn count_pending() -> Result<usize> {
    let conn = get_connection()?;
    let count: usize = conn.query_row(
        "SELECT COUNT(*) FROM notifications WHERE status = 'pending'",
        [],
        |row| row.get(0),
    )?;
    Ok(count)
}

pub fn count_approved() -> Result<usize> {
    let conn = get_connection()?;
    let count: usize = conn.query_row(
        "SELECT COUNT(*) FROM notifications WHERE status = 'approved'",
        [],
        |row| row.get(0),
    )?;
    Ok(count)
}

pub fn get_all_notifications(limit: usize) -> Result<Vec<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at, content, source_type, source_remote, source_id
         FROM notifications
         ORDER BY created_at DESC
         LIMIT ?1"
    )?;

    let notifications = stmt.query_map([limit], row_to_notification)?;
    Ok(notifications.filter_map(|r| r.ok()).collect())
}

pub fn get_by_status(status: Status, limit: usize) -> Result<Vec<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at, content, source_type, source_remote, source_id
         FROM notifications
         WHERE status = ?1
         ORDER BY created_at DESC
         LIMIT ?2"
    )?;

    let notifications = stmt.query_map(params![status.to_string(), limit], row_to_notification)?;
    Ok(notifications.filter_map(|r| r.ok()).collect())
}

pub fn get_notification_by_id(id: i64) -> Result<Option<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at, content, source_type, source_remote, source_id
         FROM notifications
         WHERE id = ?1"
    )?;

    let mut notifications = stmt.query_map([id], row_to_notification)?;
    Ok(notifications.next().and_then(|r| r.ok()))
}

pub fn update_message(id: i64, message: &str) -> Result<()> {
    let conn = get_connection()?;
    conn.execute(
        "UPDATE notifications SET message = ?1 WHERE id = ?2",
        params![message, id],
    )?;
    Ok(())
}

pub fn delete_notification(id: i64) -> Result<()> {
    let conn = get_connection()?;
    conn.execute("DELETE FROM notifications WHERE id = ?1", params![id])?;
    Ok(())
}

/// Add a notification from a remote source
pub fn add_remote_notification(
    message: &str,
    priority: Priority,
    tags: &[String],
    status: Status,
    content: Option<&str>,
    source_remote: &str,
    source_id: i64,
) -> Result<i64> {
    let conn = get_connection()?;
    let now = chrono::Utc::now().to_rfc3339();
    let tags_str = tags.join(",");

    conn.execute(
        "INSERT INTO notifications (message, priority, status, tags, created_at, content, source_type, source_remote, source_id)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, 'remote', ?7, ?8)",
        params![message, priority.to_string(), status.to_string(), tags_str, now, content, source_remote, source_id],
    )?;

    Ok(conn.last_insert_rowid())
}

/// Check if a remote notification already exists (deduplication)
pub fn remote_notification_exists(source_remote: &str, source_id: i64) -> Result<bool> {
    let conn = get_connection()?;
    let count: usize = conn.query_row(
        "SELECT COUNT(*) FROM notifications WHERE source_remote = ?1 AND source_id = ?2",
        params![source_remote, source_id],
        |row| row.get(0),
    )?;
    Ok(count > 0)
}

/// Get notifications for pull endpoint (created after a given ID)
pub fn get_notifications_since(since_id: i64, limit: usize) -> Result<Vec<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at, content, source_type, source_remote, source_id
         FROM notifications
         WHERE id > ?1 AND source_type = 'local'
         ORDER BY id ASC
         LIMIT ?2"
    )?;

    let notifications = stmt.query_map(params![since_id, limit], row_to_notification)?;
    Ok(notifications.filter_map(|r| r.ok()).collect())
}

/// Get notifications for pull endpoint filtered by tags
pub fn get_notifications_since_with_tags(since_id: i64, tags: &[String], limit: usize) -> Result<Vec<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at, content, source_type, source_remote, source_id
         FROM notifications
         WHERE id > ?1 AND source_type = 'local'
         ORDER BY id ASC
         LIMIT ?2"
    )?;

    let notifications = stmt.query_map(params![since_id, limit], row_to_notification)?;
    let all: Vec<Notification> = notifications.filter_map(|r| r.ok()).collect();

    // Filter by tags in memory (notification must have at least one matching tag, or tags filter is empty)
    if tags.is_empty() {
        return Ok(all);
    }

    let filtered = all.into_iter().filter(|n| {
        n.tags.iter().any(|t| tags.contains(t))
    }).collect();

    Ok(filtered)
}
