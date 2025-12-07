use anyhow::Result;
use colored::Colorize;
use notif_core::{add_notification, init_db, Priority, Status};

pub fn run(message: &str, priority: Priority, tags: &[String], approve: bool) -> Result<()> {
    init_db()?;

    let status = if approve { Status::Approved } else { Status::Pending };
    let id = add_notification(message, priority, tags, status)?;

    let priority_display = match priority {
        Priority::High => "high".red(),
        Priority::Normal => "normal".yellow(),
        Priority::Low => "low".dimmed(),
    };

    let status_display = if approve {
        "approved".green()
    } else {
        "pending".yellow()
    };

    let tags_display = if tags.is_empty() {
        String::new()
    } else {
        format!(" {}", tags.join(", ").dimmed())
    };

    println!(
        "Added notification #{} [{}] ({}){}: {}",
        id.to_string().cyan(),
        priority_display,
        status_display,
        tags_display,
        message
    );

    Ok(())
}
