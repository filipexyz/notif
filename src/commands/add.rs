use anyhow::Result;
use colored::Colorize;

use crate::db;
use crate::models::Priority;

pub fn run(message: &str, priority: Priority) -> Result<()> {
    db::init_db()?;

    let id = db::add_notification(message, priority)?;

    let priority_display = match priority {
        Priority::High => "high".red(),
        Priority::Normal => "normal".yellow(),
        Priority::Low => "low".dimmed(),
    };

    println!(
        "Added notification #{} [{}]: {}",
        id.to_string().cyan(),
        priority_display,
        message
    );

    Ok(())
}
