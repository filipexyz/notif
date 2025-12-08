mod commands;

use anyhow::Result;
use clap::{Parser, Subcommand};
use notif_core::Priority;

#[derive(Parser)]
#[command(name = "notif")]
#[command(about = "Notification center for LLM applications", long_about = None)]
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

        /// Auto-approve (skip UI review, inject immediately)
        #[arg(short = 'a', long)]
        approve: bool,

        /// Extended content (read with notif read <id>)
        #[arg(short = 'c', long)]
        content: Option<String>,
    },

    /// List notifications
    Ls {
        /// Filter by tags (comma-separated)
        #[arg(short = 't', long, value_delimiter = ',')]
        tags: Vec<String>,

        /// Show all notifications (not just pending)
        #[arg(short = 'a', long)]
        all: bool,

        /// Filter by status (pending, approved, dismissed, delivered)
        #[arg(short = 's', long)]
        status: Option<String>,

        /// Maximum number of notifications to show
        #[arg(short = 'n', long, default_value = "50")]
        limit: usize,
    },

    /// Approve a pending notification
    Approve {
        /// Notification ID to approve
        id: Option<i64>,

        /// Approve all pending notifications
        #[arg(long)]
        all: bool,
    },

    /// Dismiss a pending notification
    Dismiss {
        /// Notification ID to dismiss
        id: Option<i64>,

        /// Dismiss all pending notifications
        #[arg(long)]
        all: bool,
    },

    /// Hook mode (called by Claude Code)
    Hook,

    /// Clear delivered and dismissed notifications
    Clear,

    /// Read full notification content
    Read {
        /// Notification ID to read
        id: i64,
    },

    /// Setup hook in ~/.claude/settings.json and optionally create .notif.json
    Init {
        /// Tags for project config (comma-separated, creates .notif.json)
        #[arg(short = 't', long, value_delimiter = ',')]
        tags: Vec<String>,
    },

    /// Start the HTTP server for webhooks
    Server {
        /// Host to bind to
        #[arg(long, default_value = "127.0.0.1")]
        host: Option<String>,

        /// Port to listen on
        #[arg(short, long, default_value = "8787")]
        port: Option<u16>,

        /// Generate a new API key
        #[arg(long)]
        keygen: bool,
    },
}

fn main() -> Result<()> {
    let cli = Cli::parse();

    match cli.command {
        Commands::Add { message, priority, tags, approve, content } => {
            commands::add::run(&message, priority, &tags, approve, content.as_deref())
        }
        Commands::Ls { tags, all, status, limit } => commands::ls::run(&tags, all, status.as_deref(), limit),
        Commands::Approve { id, all } => commands::approve::run(id, all),
        Commands::Dismiss { id, all } => commands::dismiss::run(id, all),
        Commands::Hook => commands::hook::run(),
        Commands::Clear => commands::clear::run(),
        Commands::Read { id } => commands::read::run(id),
        Commands::Init { tags } => commands::init::run(&tags),
        Commands::Server { host, port, keygen } => {
            if keygen {
                commands::server::keygen()
            } else {
                commands::server::run(host, port)
            }
        }
    }
}
