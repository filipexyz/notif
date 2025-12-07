use anyhow::Result;
use colored::Colorize;
use notif_core::{approve, approve_all_pending, init_db};

pub fn run(id: Option<i64>, all: bool) -> Result<()> {
    init_db()?;

    if all {
        let count = approve_all_pending()?;
        if count == 0 {
            println!("{}", "No pending notifications to approve".dimmed());
        } else {
            println!(
                "Approved {} notification(s)",
                count.to_string().green()
            );
        }
    } else if let Some(id) = id {
        approve(id)?;
        println!(
            "Approved notification #{}",
            id.to_string().cyan()
        );
    } else {
        println!("{}", "Please specify an ID or use --all".red());
    }

    Ok(())
}
