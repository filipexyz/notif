// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use notif_core::{
    approve, approve_all_pending, delete_notification, dismiss, dismiss_all_pending,
    get_all_notifications, get_all_pending, get_by_status, init_db, update_message,
    load_remotes_config, add_remote, remove_remote, get_all_sync_states, get_sync_state,
    update_sync_state, add_remote_notification, remote_notification_exists,
    Notification, Status, RemoteConfig, RemoteMode, SyncState, NotifClient,
};
use serde::{Deserialize, Serialize};

#[tauri::command]
fn get_pending() -> Result<Vec<Notification>, String> {
    init_db().map_err(|e| e.to_string())?;
    get_all_pending().map_err(|e| e.to_string())
}

#[tauri::command]
fn get_history(status_filter: Option<String>, limit: usize) -> Result<Vec<Notification>, String> {
    init_db().map_err(|e| e.to_string())?;

    match status_filter.as_deref() {
        Some("pending") => get_by_status(Status::Pending, limit).map_err(|e| e.to_string()),
        Some("approved") => get_by_status(Status::Approved, limit).map_err(|e| e.to_string()),
        Some("dismissed") => get_by_status(Status::Dismissed, limit).map_err(|e| e.to_string()),
        Some("delivered") => get_by_status(Status::Delivered, limit).map_err(|e| e.to_string()),
        _ => get_all_notifications(limit).map_err(|e| e.to_string()),
    }
}

#[tauri::command]
fn approve_notification(id: i64) -> Result<(), String> {
    init_db().map_err(|e| e.to_string())?;
    approve(id).map_err(|e| e.to_string())
}

#[tauri::command]
fn dismiss_notification(id: i64) -> Result<(), String> {
    init_db().map_err(|e| e.to_string())?;
    dismiss(id).map_err(|e| e.to_string())
}

#[tauri::command]
fn edit_notification(id: i64, message: String) -> Result<(), String> {
    init_db().map_err(|e| e.to_string())?;
    update_message(id, &message).map_err(|e| e.to_string())
}

#[tauri::command]
fn approve_all() -> Result<usize, String> {
    init_db().map_err(|e| e.to_string())?;
    approve_all_pending().map_err(|e| e.to_string())
}

#[tauri::command]
fn dismiss_all() -> Result<usize, String> {
    init_db().map_err(|e| e.to_string())?;
    dismiss_all_pending().map_err(|e| e.to_string())
}

#[tauri::command]
fn delete_notif(id: i64) -> Result<(), String> {
    init_db().map_err(|e| e.to_string())?;
    delete_notification(id).map_err(|e| e.to_string())
}

// Remote management types for UI
#[derive(Debug, Clone, Serialize, Deserialize)]
struct RemoteInfo {
    name: String,
    url: String,
    mode: String,
    tags: Vec<String>,
    auto_approve: bool,
    last_synced_id: i64,
    last_synced_at: Option<String>,
}

#[tauri::command]
fn get_remotes() -> Result<Vec<RemoteInfo>, String> {
    init_db().map_err(|e| e.to_string())?;
    let config = load_remotes_config().map_err(|e| e.to_string())?;
    let sync_states = get_all_sync_states().map_err(|e| e.to_string())?;

    let remotes = config.remotes.iter().map(|r| {
        let sync = sync_states.iter().find(|s| s.remote_name == r.name);
        RemoteInfo {
            name: r.name.clone(),
            url: r.url.clone(),
            mode: match r.mode {
                RemoteMode::Store => "store".to_string(),
                RemoteMode::Passthrough => "passthrough".to_string(),
            },
            tags: r.tags.clone(),
            auto_approve: r.auto_approve,
            last_synced_id: sync.map(|s| s.last_synced_id).unwrap_or(0),
            last_synced_at: sync.and_then(|s| s.last_synced_at.clone()),
        }
    }).collect();

    Ok(remotes)
}

#[tauri::command]
fn add_remote_config(
    name: String,
    url: String,
    api_key: String,
    mode: String,
    tags: Vec<String>,
    auto_approve: bool,
) -> Result<(), String> {
    let remote_mode = match mode.as_str() {
        "passthrough" => RemoteMode::Passthrough,
        _ => RemoteMode::Store,
    };

    let remote = RemoteConfig {
        name,
        url,
        api_key,
        mode: remote_mode,
        tags,
        auto_approve,
    };

    add_remote(remote).map_err(|e| e.to_string())
}

#[tauri::command]
fn remove_remote_config(name: String) -> Result<bool, String> {
    remove_remote(&name).map_err(|e| e.to_string())
}

#[tauri::command]
async fn test_remote(name: String) -> Result<bool, String> {
    let config = load_remotes_config().map_err(|e| e.to_string())?;
    let remote = config.get_remote(&name).ok_or("Remote not found")?;

    let client = NotifClient::new().map_err(|e| e.to_string())?;
    client.test_connection(remote).await.map_err(|e| e.to_string())?;
    Ok(true)
}

#[tauri::command]
async fn pull_remote(name: String) -> Result<usize, String> {
    init_db().map_err(|e| e.to_string())?;
    let config = load_remotes_config().map_err(|e| e.to_string())?;
    let remote = config.get_remote(&name).ok_or("Remote not found")?.clone();

    let sync = get_sync_state(&name).map_err(|e| e.to_string())?;
    let client = NotifClient::new().map_err(|e| e.to_string())?;
    let response = client.pull(&remote, sync.last_synced_id).await.map_err(|e| e.to_string())?;

    let mut count = 0;
    for notif in &response.notifications {
        if remote_notification_exists(&remote.name, notif.id).map_err(|e| e.to_string())? {
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
        ).map_err(|e| e.to_string())?;
        count += 1;
    }

    update_sync_state(&remote.name, response.last_id).map_err(|e| e.to_string())?;
    Ok(count)
}

#[tauri::command]
async fn pull_all_remotes() -> Result<usize, String> {
    init_db().map_err(|e| e.to_string())?;
    let config = load_remotes_config().map_err(|e| e.to_string())?;

    let mut total = 0;
    let client = NotifClient::new().map_err(|e| e.to_string())?;

    for remote in &config.remotes {
        let sync = get_sync_state(&remote.name).map_err(|e| e.to_string())?;

        if let Ok(response) = client.pull(remote, sync.last_synced_id).await {
            for notif in &response.notifications {
                if remote_notification_exists(&remote.name, notif.id).unwrap_or(true) {
                    continue;
                }

                let status = if remote.auto_approve {
                    Status::Approved
                } else {
                    Status::Pending
                };

                if add_remote_notification(
                    &notif.message,
                    notif.priority,
                    &notif.tags,
                    status,
                    notif.content.as_deref(),
                    &remote.name,
                    notif.id,
                ).is_ok() {
                    total += 1;
                }
            }
            let _ = update_sync_state(&remote.name, response.last_id);
        }
    }

    Ok(total)
}

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            get_pending,
            get_history,
            approve_notification,
            dismiss_notification,
            edit_notification,
            approve_all,
            dismiss_all,
            delete_notif,
            get_remotes,
            add_remote_config,
            remove_remote_config,
            test_remote,
            pull_remote,
            pull_all_remotes,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
