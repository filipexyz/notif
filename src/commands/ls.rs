use anyhow::Result;
use colored::Colorize;

use crate::db;
use crate::models::Priority;

pub fn run() -> Result<()> {
    db::init_db()?;

    let notifications = db::get_all_pending()?;

    if notifications.is_empty() {
        println!("{}", "No pending notifications".dimmed());
        return Ok(());
    }

    println!("{}", "Pending notifications:".bold());
    println!();

    for notif in &notifications {
        let priority_badge = match notif.priority {
            Priority::High => "[HIGH]".red().bold(),
            Priority::Normal => "[NORMAL]".yellow(),
            Priority::Low => "[LOW]".dimmed(),
        };

        let id_display = format!("#{}", notif.id).cyan();

        println!("  {} {} {}", id_display, priority_badge, notif.message);
    }

    println!();
    println!(
        "{} notification(s) pending",
        notifications.len().to_string().cyan()
    );

    Ok(())
}
