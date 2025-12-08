use anyhow::Result;
use colored::Colorize;
use notif_core::{
    get_all_notifications, get_all_pending_filtered, get_by_status, init_db, FilterMode, Priority,
    Status, TagFilter,
};

pub fn run(filter_tags: &[String], all: bool, status_filter: Option<&str>, limit: usize) -> Result<()> {
    init_db()?;

    let filter = if filter_tags.is_empty() {
        None
    } else {
        Some(TagFilter {
            tags: filter_tags.to_vec(),
            mode: FilterMode::Include,
        })
    };

    let notifications = if all {
        // Show all notifications
        get_all_notifications(limit)?
    } else if let Some(status_str) = status_filter {
        // Filter by specific status
        let status = match status_str.to_lowercase().as_str() {
            "pending" => Status::Pending,
            "approved" => Status::Approved,
            "dismissed" => Status::Dismissed,
            "delivered" => Status::Delivered,
            _ => {
                eprintln!("Unknown status: {}. Use: pending, approved, dismissed, delivered", status_str);
                return Ok(());
            }
        };
        get_by_status(status, limit)?
    } else {
        // Default: show pending only
        get_all_pending_filtered(filter.as_ref())?
    };

    // Apply tag filter for --all mode (already applied for pending mode)
    let notifications: Vec<_> = if all && filter.is_some() {
        let f = filter.as_ref().unwrap();
        notifications.into_iter().filter(|n| f.matches(&n.tags)).collect()
    } else {
        notifications
    };

    if notifications.is_empty() {
        let status_msg = if all {
            "No notifications"
        } else if status_filter.is_some() {
            "No notifications with that status"
        } else {
            "No pending notifications"
        };
        println!("{}", status_msg.dimmed());
        return Ok(());
    }

    let title = if all {
        "All notifications:"
    } else if let Some(s) = status_filter {
        match s.to_lowercase().as_str() {
            "pending" => "Pending notifications:",
            "approved" => "Approved notifications:",
            "dismissed" => "Dismissed notifications:",
            "delivered" => "Delivered notifications:",
            _ => "Notifications:",
        }
    } else {
        "Pending notifications:"
    };

    println!("{}", title.bold());
    println!();

    for notif in &notifications {
        let priority_badge = match notif.priority {
            Priority::High => "[HIGH]".red().bold(),
            Priority::Normal => "[NORMAL]".yellow(),
            Priority::Low => "[LOW]".dimmed(),
        };

        let status_badge = match notif.status {
            Status::Pending => "[PENDING]".yellow(),
            Status::Approved => "[APPROVED]".green(),
            Status::Dismissed => "[DISMISSED]".dimmed(),
            Status::Delivered => "[DELIVERED]".blue(),
        };

        let id_display = format!("#{}", notif.id).cyan();

        let tags_display = if notif.tags.is_empty() {
            String::new()
        } else {
            format!(" {}", notif.tags.join(", ").dimmed())
        };

        println!(
            "  {} {} {}{} {}",
            id_display, priority_badge, status_badge, tags_display, notif.message
        );

        // Show content hint if content exists
        if let Some(tokens) = notif.content_tokens() {
            println!(
                "    {}",
                format!("read full content (~{} tokens) with: notif read {}", tokens, notif.id).dimmed()
            );
        }
    }

    println!();
    println!(
        "{} notification(s)",
        notifications.len().to_string().cyan()
    );

    Ok(())
}
