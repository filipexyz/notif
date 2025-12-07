mod commands;
mod config;
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

        /// Tags (comma-separated)
        #[arg(short = 't', long, value_delimiter = ',')]
        tags: Vec<String>,
    },

    /// List pending notifications
    Ls {
        /// Filter by tags (comma-separated)
        #[arg(short = 't', long, value_delimiter = ',')]
        tags: Vec<String>,
    },

    /// Hook mode (called by Claude Code)
    Hook,

    /// Clear delivered notifications
    Clear,

    /// Setup hook in ~/.claude/settings.json and optionally create .notif.json
    Init {
        /// Tags for project config (comma-separated, creates .notif.json)
        #[arg(short = 't', long, value_delimiter = ',')]
        tags: Vec<String>,
    },
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::Add { message, priority, tags } => {
            commands::add::run(&message, priority, &tags)
        }
        Commands::Ls { tags } => commands::ls::run(&tags),
        Commands::Hook => commands::hook::run(),
        Commands::Clear => commands::clear::run(),
        Commands::Init { tags } => commands::init::run(&tags),
    }
}
