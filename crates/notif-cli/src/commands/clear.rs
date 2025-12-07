use anyhow::Result;
use colored::Colorize;
use notif_core::{clear_delivered, clear_dismissed, init_db};

pub fn run() -> Result<()> {
    init_db()?;

    let delivered = clear_delivered()?;
    let dismissed = clear_dismissed()?;

    if delivered == 0 && dismissed == 0 {
        println!("{}", "No notifications to clear".dimmed());
    } else {
        if delivered > 0 {
            println!(
                "Cleared {} delivered notification(s)",
                delivered.to_string().green()
            );
        }
        if dismissed > 0 {
            println!(
                "Cleared {} dismissed notification(s)",
                dismissed.to_string().green()
            );
        }
    }

    Ok(())
}
