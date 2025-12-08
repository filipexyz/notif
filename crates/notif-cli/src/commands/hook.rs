use anyhow::Result;
use notif_core::{
    add_remote_notification, get_approved_filtered, get_sync_state, init_db,
    load_project_config, load_remotes_config, mark_delivered, remote_notification_exists,
    update_sync_state, Notification, NotifClient, RemoteMode, Status,
};
use std::io::{self, Read};

const MAX_NOTIFICATIONS: usize = 3;

pub fn run() -> Result<()> {
    // Read stdin (Claude sends JSON context, but we don't need to parse it)
    let mut _input = String::new();
    let _ = io::stdin().read_to_string(&mut _input);

    init_db()?;

    // Load project config for tag filtering
    let filter = load_project_config();

    // Load remotes config and pull from remotes
    let remotes_config = load_remotes_config()?;
    let mut passthrough_notifs: Vec<Notification> = Vec::new();

    if !remotes_config.remotes.is_empty() {
        let rt = tokio::runtime::Runtime::new()?;
        rt.block_on(async {
            let client = match NotifClient::new() {
                Ok(c) => c,
                Err(_) => return, // Silently skip if client fails
            };

            for remote in &remotes_config.remotes {
                let sync = match get_sync_state(&remote.name) {
                    Ok(s) => s,
                    Err(_) => continue,
                };

                let response = match client.pull(remote, sync.last_synced_id).await {
                    Ok(r) => r,
                    Err(_) => continue, // Skip unreachable remotes
                };

                if response.notifications.is_empty() {
                    continue;
                }

                match remote.mode {
                    RemoteMode::Store => {
                        // Store in local DB
                        for notif in &response.notifications {
                            // Check for duplicates
                            if let Ok(true) = remote_notification_exists(&remote.name, notif.id) {
                                continue;
                            }

                            let status = if remote.auto_approve {
                                Status::Approved
                            } else {
                                Status::Pending
                            };

                            let _ = add_remote_notification(
                                &notif.message,
                                notif.priority,
                                &notif.tags,
                                status,
                                notif.content.as_deref(),
                                &remote.name,
                                notif.id,
                            );
                        }
                    }
                    RemoteMode::Passthrough => {
                        // Collect for direct output
                        for notif in &response.notifications {
                            passthrough_notifs.push(notif.to_notification(&remote.name));
                        }
                    }
                }

                // Update sync state
                let _ = update_sync_state(&remote.name, response.last_id);
            }
        });
    }

    // Get local APPROVED notifications
    let mut local_notifs = get_approved_filtered(MAX_NOTIFICATIONS, filter.as_ref())?;

    // Combine local approved with passthrough (limit total output)
    let remaining = MAX_NOTIFICATIONS.saturating_sub(local_notifs.len());
    passthrough_notifs.truncate(remaining);

    let has_local = !local_notifs.is_empty();
    let has_passthrough = !passthrough_notifs.is_empty();

    if !has_local && !has_passthrough {
        // No output = no context injected
        return Ok(());
    }

    // Output plain text that Claude will see
    println!("Pending notifications:");

    // Output local notifications
    for notif in &local_notifs {
        print_notification(notif);
    }

    // Output passthrough notifications (mark with remote source)
    for notif in &passthrough_notifs {
        print_notification(notif);
    }

    // Mark local notifications as delivered
    if has_local {
        let ids: Vec<i64> = local_notifs.drain(..).map(|n| n.id).collect();
        mark_delivered(&ids)?;
    }

    Ok(())
}

fn print_notification(notif: &Notification) {
    let source_prefix = match &notif.source_remote {
        Some(remote) => format!("@{} ", remote),
        None => String::new(),
    };

    if notif.tags.is_empty() {
        println!("- {}{}", source_prefix, notif.message);
    } else {
        println!("- {}[{}] {}", source_prefix, notif.tags.join(", "), notif.message);
    }

    // Show content hint if content exists
    if let Some(tokens) = notif.content_tokens() {
        println!("  (read full content ~{} tokens with: notif read {})", tokens, notif.id);
    }
}
