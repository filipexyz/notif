use anyhow::Result;
use std::io::{self, Read};

use crate::db;

const MAX_NOTIFICATIONS: usize = 3;

pub fn run() -> Result<()> {
    // Read stdin (Claude sends JSON context, but we don't need to parse it)
    let mut _input = String::new();
    let _ = io::stdin().read_to_string(&mut _input);

    db::init_db()?;

    let notifications = db::get_pending(MAX_NOTIFICATIONS)?;

    if notifications.is_empty() {
        // No output = no context injected
        return Ok(());
    }

    // Output plain text that Claude will see
    println!("Pending notifications:");
    for notif in &notifications {
        println!("- {}", notif.message);
    }

    // Mark as delivered
    let ids: Vec<i64> = notifications.iter().map(|n| n.id).collect();
    db::mark_delivered(&ids)?;

    Ok(())
}
