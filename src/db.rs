use anyhow::{Context, Result};
use directories::ProjectDirs;
use rusqlite::{params, Connection};
use std::fs;
use std::path::PathBuf;

use crate::models::{Notification, Priority, TagFilter};

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
            delivered_at TEXT
        )",
        [],
    )?;

    // Migration: add tags column if it doesn't exist (for existing DBs)
    let _ = conn.execute("ALTER TABLE notifications ADD COLUMN tags TEXT NOT NULL DEFAULT ''", []);

    conn.execute(
        "CREATE INDEX IF NOT EXISTS idx_status ON notifications(status)",
        [],
    )?;

    conn.execute(
        "CREATE INDEX IF NOT EXISTS idx_priority ON notifications(priority)",
        [],
    )?;

    Ok(())
}

pub fn add_notification(message: &str, priority: Priority, tags: &[String]) -> Result<i64> {
    let conn = get_connection()?;
    let now = chrono::Utc::now().to_rfc3339();
    let tags_str = tags.join(",");

    conn.execute(
        "INSERT INTO notifications (message, priority, status, tags, created_at) VALUES (?1, ?2, 'pending', ?3, ?4)",
        params![message, priority.to_string(), tags_str, now],
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

pub fn get_pending_filtered(limit: usize, filter: Option<&TagFilter>) -> Result<Vec<Notification>> {
    let conn = get_connection()?;

    let mut stmt = conn.prepare(
        "SELECT id, message, priority, status, tags, created_at, delivered_at
         FROM notifications
         WHERE status = 'pending'
         ORDER BY
            CASE priority
                WHEN 'high' THEN 1
                WHEN 'normal' THEN 2
                WHEN 'low' THEN 3
            END,
            created_at ASC"
    )?;

    let notifications = stmt.query_map([], |row| {
        let priority_str: String = row.get(2)?;
        let priority = priority_str.parse().unwrap_or(Priority::Normal);
        let tags_str: String = row.get(4)?;

        Ok(Notification {
            id: row.get(0)?,
            message: row.get(1)?,
            priority,
            status: row.get(3)?,
            tags: parse_tags(&tags_str),
            created_at: row.get(5)?,
            delivered_at: row.get(6)?,
        })
    })?;

    let all: Vec<Notification> = notifications.filter_map(|r| r.ok()).collect();

    // Apply tag filter in memory
    let filtered: Vec<Notification> = match filter {
        Some(f) => all.into_iter().filter(|n| f.matches(&n.tags)).collect(),
        None => all,
    };

    Ok(filtered.into_iter().take(limit).collect())
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

pub fn mark_delivered(ids: &[i64]) -> Result<()> {
    if ids.is_empty() {
        return Ok(());
    }

    let conn = get_connection()?;
    let now = chrono::Utc::now().to_rfc3339();

    let placeholders: Vec<String> = ids.iter().map(|_| "?".to_string()).collect();
    let sql = format!(
        "UPDATE notifications SET status = 'delivered', delivered_at = ?1 WHERE id IN ({})",
        placeholders.join(", ")
    );

    let mut params: Vec<Box<dyn rusqlite::ToSql>> = vec![Box::new(now)];
    for id in ids {
        params.push(Box::new(*id));
    }

    conn.execute(&sql, rusqlite::params_from_iter(params.iter().map(|p| p.as_ref())))?;

    Ok(())
}

pub fn clear_delivered() -> Result<usize> {
    let conn = get_connection()?;
    let deleted = conn.execute("DELETE FROM notifications WHERE status = 'delivered'", [])?;
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
