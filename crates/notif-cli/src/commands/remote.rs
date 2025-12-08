use anyhow::{bail, Result};
use colored::Colorize;
use notif_core::{
    add_remote, get_all_sync_states, get_sync_state, init_db, load_remotes_config,
    remove_remote, update_sync_state, NotifClient, RemoteConfig, RemoteMode,
    add_remote_notification, remote_notification_exists, Status,
};

pub fn list() -> Result<()> {
    let config = load_remotes_config()?;

    if config.remotes.is_empty() {
        println!("No remotes configured.");
        println!("Add one with: notif remote add <name>");
        return Ok(());
    }

    println!("{}", "Configured remotes:".bold());
    for remote in &config.remotes {
        let mode = match remote.mode {
            RemoteMode::Store => "store",
            RemoteMode::Passthrough => "passthrough",
        };
        println!(
            "  {} {} {}",
            remote.name.cyan(),
            format!("({})", mode).dimmed(),
            remote.url.dimmed()
        );
        if !remote.tags.is_empty() {
            println!("    tags: {}", remote.tags.join(", ").yellow());
        }
        if remote.auto_approve {
            println!("    auto-approve: {}", "yes".green());
        }
    }

    Ok(())
}

pub fn add(
    name: &str,
    url: &str,
    api_key: &str,
    mode: Option<&str>,
    tags: &[String],
    auto_approve: bool,
) -> Result<()> {
    let mode = match mode {
        Some("store") | None => RemoteMode::Store,
        Some("passthrough") => RemoteMode::Passthrough,
        Some(m) => bail!("Invalid mode '{}'. Use 'store' or 'passthrough'.", m),
    };

    let remote = RemoteConfig {
        name: name.to_string(),
        url: url.to_string(),
        api_key: api_key.to_string(),
        mode,
        tags: tags.to_vec(),
        auto_approve,
    };

    add_remote(remote)?;
    println!("Added remote: {}", name.cyan());

    Ok(())
}

pub fn rm(name: &str) -> Result<()> {
    if remove_remote(name)? {
        println!("Removed remote: {}", name.cyan());
    } else {
        bail!("Remote '{}' not found", name);
    }
    Ok(())
}

pub fn test(name: &str) -> Result<()> {
    let config = load_remotes_config()?;
    let Some(remote) = config.get_remote(name) else {
        bail!("Remote '{}' not found", name);
    };

    println!("Testing connection to {}...", remote.name.cyan());

    let rt = tokio::runtime::Runtime::new()?;
    rt.block_on(async {
        let client = NotifClient::new()?;
        match client.test_connection(remote).await {
            Ok(()) => {
                println!("{} Connection successful!", "✓".green());
                Ok(())
            }
            Err(e) => {
                println!("{} Connection failed: {}", "✗".red(), e);
                Err(e)
            }
        }
    })
}

pub fn status() -> Result<()> {
    init_db()?;
    let config = load_remotes_config()?;
    let sync_states = get_all_sync_states()?;

    if config.remotes.is_empty() {
        println!("No remotes configured.");
        return Ok(());
    }

    println!("{}", "Remote sync status:".bold());
    for remote in &config.remotes {
        let state = sync_states
            .iter()
            .find(|s| s.remote_name == remote.name)
            .map(|s| {
                format!(
                    "last_id={}, synced={}",
                    s.last_synced_id,
                    s.last_synced_at.as_deref().unwrap_or("never")
                )
            })
            .unwrap_or_else(|| "never synced".to_string());

        let mode = match remote.mode {
            RemoteMode::Store => "store",
            RemoteMode::Passthrough => "passthrough",
        };

        println!(
            "  {} {} {}",
            remote.name.cyan(),
            format!("({})", mode).dimmed(),
            state.dimmed()
        );
    }

    Ok(())
}

pub fn pull(name: Option<&str>) -> Result<()> {
    init_db()?;
    let config = load_remotes_config()?;

    let remotes: Vec<&RemoteConfig> = match name {
        Some(n) => {
            let Some(remote) = config.get_remote(n) else {
                bail!("Remote '{}' not found", n);
            };
            vec![remote]
        }
        None => config.remotes.iter().collect(),
    };

    if remotes.is_empty() {
        println!("No remotes configured.");
        return Ok(());
    }

    let rt = tokio::runtime::Runtime::new()?;
    rt.block_on(async {
        let client = NotifClient::new()?;

        for remote in remotes {
            let sync = get_sync_state(&remote.name)?;
            print!("Pulling from {}... ", remote.name.cyan());

            match client.pull(remote, sync.last_synced_id).await {
                Ok(response) => {
                    let count = response.notifications.len();
                    if count == 0 {
                        println!("{}", "no new notifications".dimmed());
                    } else {
                        match remote.mode {
                            RemoteMode::Store => {
                                // Store notifications locally
                                let mut stored = 0;
                                for notif in &response.notifications {
                                    // Check for duplicates
                                    if remote_notification_exists(&remote.name, notif.id)? {
                                        continue;
                                    }

                                    let status = if remote.auto_approve {
                                        Status::Approved
                                    } else {
                                        Status::Pending
                                    };

                                    add_remote_notification(
                                        &notif.message,
                                        notif.priority,
                                        &notif.tags,
                                        status,
                                        notif.content.as_deref(),
                                        &remote.name,
                                        notif.id,
                                    )?;
                                    stored += 1;
                                }

                                println!(
                                    "{} {} stored (last_id={})",
                                    "✓".green(),
                                    stored,
                                    response.last_id
                                );
                            }
                            RemoteMode::Passthrough => {
                                // Just show count, don't store
                                println!(
                                    "{} {} available (passthrough)",
                                    "✓".green(),
                                    count
                                );
                            }
                        }

                        // Update sync state
                        update_sync_state(&remote.name, response.last_id)?;
                    }
                }
                Err(e) => {
                    println!("{} {}", "✗".red(), e);
                }
            }
        }
        Ok(())
    })
}
