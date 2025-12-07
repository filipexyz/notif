use anyhow::Result;
use colored::Colorize;

use crate::db;
use crate::models::Priority;

pub fn run(message: &str, priority: Priority, tags: &[String]) -> Result<()> {
    db::init_db()?;

    let id = db::add_notification(message, priority, tags)?;

    let priority_display = match priority {
        Priority::High => "high".red(),
        Priority::Normal => "normal".yellow(),
        Priority::Low => "low".dimmed(),
    };

    let tags_display = if tags.is_empty() {
        String::new()
    } else {
        format!(" {}", tags.join(", ").dimmed())
    };

    println!(
        "Added notification #{} [{}]{}: {}",
        id.to_string().cyan(),
        priority_display,
        tags_display,
        message
    );

    Ok(())
}
