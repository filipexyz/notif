use anyhow::Result;
use colored::Colorize;
use notif_core::{add_notification_with_content, init_db, Priority, Status};

pub fn run(message: &str, priority: Priority, tags: &[String], approve: bool, content: Option<&str>) -> Result<()> {
    init_db()?;

    let status = if approve { Status::Approved } else { Status::Pending };
    let id = add_notification_with_content(message, priority, tags, status, content)?;

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

    if content.is_some() {
        let tokens = content.map(|c| (c.len() + 3) / 4).unwrap_or(0);
        println!(
            "  {}",
            format!("with content (~{} tokens) - read with: notif read {}", tokens, id).dimmed()
        );
    }

    Ok(())
}
