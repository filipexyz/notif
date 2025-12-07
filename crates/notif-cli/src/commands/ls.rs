use anyhow::Result;
use colored::Colorize;
use notif_core::{get_all_pending_filtered, init_db, FilterMode, Priority, Status, TagFilter};

pub fn run(filter_tags: &[String]) -> Result<()> {
    init_db()?;

    let filter = if filter_tags.is_empty() {
        None
    } else {
        Some(TagFilter {
            tags: filter_tags.to_vec(),
            mode: FilterMode::Include,
        })
    };

    let notifications = get_all_pending_filtered(filter.as_ref())?;

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

        println!("  {} {} {}{} {}", id_display, priority_badge, status_badge, tags_display, notif.message);
    }

    println!();
    println!(
        "{} notification(s) pending",
        notifications.len().to_string().cyan()
    );

    Ok(())
}
