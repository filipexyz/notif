use anyhow::{bail, Result};
use colored::Colorize;
use notif_core::get_notification_by_id;

pub fn run(id: i64) -> Result<()> {
    let Some(notif) = get_notification_by_id(id)? else {
        bail!("Notification #{} not found", id);
    };

    // Header
    println!(
        "{} {} [{}] {}",
        format!("#{}", notif.id).cyan(),
        notif.priority.to_string().to_uppercase().yellow(),
        notif.status.to_string().white(),
        if notif.tags.is_empty() {
            String::new()
        } else {
            format!("[{}]", notif.tags.join(", ")).magenta().to_string()
        }
    );

    // Message
    println!("\n{}", "Message:".bold());
    println!("{}", notif.message);

    // Content
    if let Some(ref content) = notif.content {
        println!("\n{}", "Content:".bold());
        println!("{}", content);
        if let Some(tokens) = notif.content_tokens() {
            println!("\n{}", format!("(~{} tokens)", tokens).dimmed());
        }
    } else {
        println!("\n{}", "(no extended content)".dimmed());
    }

    // Metadata
    println!("\n{}", "Created:".dimmed());
    println!("{}", notif.created_at.dimmed());
    if let Some(ref delivered) = notif.delivered_at {
        println!("{}", "Delivered:".dimmed());
        println!("{}", delivered.dimmed());
    }

    Ok(())
}
