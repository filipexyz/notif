use futures_util::StreamExt;
use notifsh::{Notif, SubscribeOptions};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::sync::Arc;
use tauri::{AppHandle, Emitter, State};
use uuid::Uuid;

/// Notification level
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Level {
    Info,
    Warn,
    Error,
}

/// Notification payload
#[derive(Debug, Clone, Serialize)]
pub struct Notification {
    pub id: String,
    pub title: String,
    pub body: String,
    pub level: Level,
}

/// Permission request from Claude Code hook
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PermissionRequest {
    pub tool_name: Option<String>,
    pub tool_input: Option<serde_json::Value>,
    pub session_id: Option<String>,
    pub cwd: Option<String>,
}

struct NotifClient(Arc<Notif>);

#[tauri::command]
async fn send_notification(
    title: String,
    body: String,
    level: String,
    client: State<'_, NotifClient>,
) -> Result<String, String> {
    let client = &client.0;

    let level_enum = match level.as_str() {
        "warn" => Level::Warn,
        "error" => Level::Error,
        _ => Level::Info,
    };

    let id = Uuid::new_v4().to_string();

    let notification = Notification {
        id: id.clone(),
        title,
        body,
        level: level_enum,
    };

    client
        .emit("desktop.hub.notify", json!(notification))
        .await
        .map_err(|e| e.to_string())?;

    Ok(id)
}

#[tauri::command]
fn get_env_status() -> Result<String, String> {
    match std::env::var("NOTIF_API_KEY") {
        Ok(_) => Ok("connected".to_string()),
        Err(_) => Ok("not_configured".to_string()),
    }
}

/// Respond to a permission request
#[tauri::command]
async fn respond_permission(
    decision: String,
    message: Option<String>,
    client: State<'_, NotifClient>,
) -> Result<(), String> {
    let client = &client.0;

    // Build the hook response format
    let response = if decision == "allow" {
        json!({
            "hookSpecificOutput": {
                "hookEventName": "PermissionRequest",
                "decision": {
                    "behavior": "allow"
                }
            }
        })
    } else {
        json!({
            "hookSpecificOutput": {
                "hookEventName": "PermissionRequest",
                "decision": {
                    "behavior": "deny",
                    "message": message.unwrap_or_else(|| "Denied by user".to_string())
                }
            }
        })
    };

    client
        .emit("claude.permission.response", response)
        .await
        .map_err(|e| e.to_string())?;

    Ok(())
}

/// Subscribe to permission requests and forward to frontend
async fn start_permission_listener(app: AppHandle, client: Arc<Notif>) {
    loop {
        match client
            .subscribe_with_options(
                &["claude.permission.request"],
                SubscribeOptions::new().auto_ack(true).from("latest"),
            )
            .await
        {
            Ok(mut stream) => {
                while let Some(result) = stream.next().await {
                    if let Ok(event) = result {
                        // Forward to frontend
                        let _ = app.emit("permission_request", &event.data);
                    }
                }
            }
            Err(e) => {
                eprintln!("Permission subscription error: {}", e);
            }
        }
        // Reconnect after delay
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let client = Arc::new(Notif::from_env().expect("NOTIF_API_KEY must be set"));

    tauri::Builder::default()
        .manage(NotifClient(client.clone()))
        .invoke_handler(tauri::generate_handler![
            send_notification,
            get_env_status,
            respond_permission
        ])
        .setup(move |app| {
            let handle = app.handle().clone();
            let client_clone = client.clone();

            // Start permission listener in background
            tauri::async_runtime::spawn(async move {
                start_permission_listener(handle, client_clone).await;
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
