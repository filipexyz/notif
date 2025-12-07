use anyhow::Result;
use colored::Colorize;

use crate::db;

pub fn run() -> Result<()> {
    db::init_db()?;

    let deleted = db::clear_delivered()?;

    if deleted == 0 {
        println!("{}", "No delivered notifications to clear".dimmed());
    } else {
        println!(
            "Cleared {} delivered notification(s)",
            deleted.to_string().green()
        );
    }

    Ok(())
}
