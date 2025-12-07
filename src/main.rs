mod commands;
mod db;
mod models;

use anyhow::Result;
use clap::{Parser, Subcommand};

use crate::models::Priority;

#[derive(Parser)]
#[command(name = "notif")]
#[command(about = "Claude Code notification center", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Add a new notification
    Add {
        /// The notification message
        message: String,

        /// Priority level (low, normal, high)
        #[arg(short, long, default_value = "normal")]
        priority: Priority,
    },

    /// List pending notifications
    Ls,

    /// Hook mode (called by Claude Code)
    Hook,

    /// Clear delivered notifications
    Clear,

    /// Setup hook in ~/.claude/settings.json
    Init,
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::Add { message, priority } => {
            commands::add::run(&message, priority)
        }
        Commands::Ls => commands::ls::run(),
        Commands::Hook => commands::hook::run(),
        Commands::Clear => commands::clear::run(),
        Commands::Init => commands::init::run(),
    }
}
