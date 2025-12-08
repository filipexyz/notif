use anyhow::Result;
use notif_core::{get_approved_filtered, init_db, load_project_config, mark_delivered};
use std::io::{self, Read};

const MAX_NOTIFICATIONS: usize = 3;

pub fn run() -> Result<()> {
    // Read stdin (Claude sends JSON context, but we don't need to parse it)
    let mut _input = String::new();
    let _ = io::stdin().read_to_string(&mut _input);

    init_db()?;

    // Load project config for tag filtering
    let filter = load_project_config();

    // Get APPROVED notifications only (not pending)
    let notifications = get_approved_filtered(MAX_NOTIFICATIONS, filter.as_ref())?;

    if notifications.is_empty() {
        // No output = no context injected
        return Ok(());
    }

    // Output plain text that Claude will see
    println!("Pending notifications:");
    for notif in &notifications {
        if notif.tags.is_empty() {
            println!("- {}", notif.message);
        } else {
            println!("- [{}] {}", notif.tags.join(", "), notif.message);
        }
        // Show content hint if content exists
        if let Some(tokens) = notif.content_tokens() {
            println!("  (read full content ~{} tokens with: notif read {})", tokens, notif.id);
        }
    }

    // Mark as delivered
    let ids: Vec<i64> = notifications.iter().map(|n| n.id).collect();
    mark_delivered(&ids)?;

    Ok(())
}
