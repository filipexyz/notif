use anyhow::Result;
use colored::Colorize;
use notif_core::{dismiss, dismiss_all_pending, init_db};

pub fn run(id: Option<i64>, all: bool) -> Result<()> {
    init_db()?;

    if all {
        let count = dismiss_all_pending()?;
        if count == 0 {
            println!("{}", "No pending notifications to dismiss".dimmed());
        } else {
            println!(
                "Dismissed {} notification(s)",
                count.to_string().yellow()
            );
        }
    } else if let Some(id) = id {
        dismiss(id)?;
        println!(
            "Dismissed notification #{}",
            id.to_string().cyan()
        );
    } else {
        println!("{}", "Please specify an ID or use --all".red());
    }

    Ok(())
}
