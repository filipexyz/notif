use futures_util::StreamExt;
use notifsh::{Notif, SubscribeOptions};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::sync::Arc;
use tauri::{AppHandle, Emitter, Manager, State};

/// Notification payload (matches hub's format)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Notification {
    pub id: String,
    pub title: String,
    pub body: String,
    pub level: String,
}

/// Permission request from Claude Code
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PermissionRequest {
    pub tool_name: Option<String>,
    pub tool_input: Option<serde_json::Value>,
    pub session_id: Option<String>,
    pub cwd: Option<String>,
}

struct NotifClient(Arc<Notif>);

/// Start notifications subscription
async fn start_notification_subscription(app: AppHandle, client: Arc<Notif>) {
    loop {
        match client
            .subscribe_with_options(
                &["desktop.hub.notify"],
                SubscribeOptions::new().auto_ack(true).from("latest"),
            )
            .await
        {
            Ok(mut stream) => {
                while let Some(result) = stream.next().await {
                    if let Ok(event) = result {
                        if let Ok(notification) =
                            serde_json::from_value::<Notification>(event.data)
                        {
                            let _ = app.emit("notification", &notification);
                        }
                    }
                }
            }
            Err(e) => eprintln!("Notification subscription error: {}", e),
        }
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

/// Start permission subscription
async fn start_permission_subscription(app: AppHandle, client: Arc<Notif>) {
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
                        let _ = app.emit("permission_request", &event.data);
                    }
                }
            }
            Err(e) => eprintln!("Permission subscription error: {}", e),
        }
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

/// Respond to a permission request
#[tauri::command]
async fn respond_permission(
    decision: String,
    message: Option<String>,
    session_id: Option<String>,
    client: State<'_, NotifClient>,
) -> Result<(), String> {
    let response = if decision == "allow" {
        json!({
            "session_id": session_id,
            "hookSpecificOutput": {
                "hookEventName": "PermissionRequest",
                "decision": { "behavior": "allow" }
            }
        })
    } else {
        json!({
            "session_id": session_id,
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
        .0
        .emit("claude.permission.response", response)
        .await
        .map_err(|e| e.to_string())?;

    Ok(())
}

#[tauri::command]
async fn show_window(window: tauri::Window) {
    let _ = window.show();
    let _ = window.set_focus();
}

#[tauri::command]
async fn hide_window(window: tauri::Window) {
    let _ = window.hide();
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let client = Arc::new(Notif::from_env().expect("NOTIF_API_KEY must be set"));

    tauri::Builder::default()
        .manage(NotifClient(client.clone()))
        .invoke_handler(tauri::generate_handler![
            show_window,
            hide_window,
            respond_permission
        ])
        .setup(move |app| {
            let handle = app.handle().clone();
            let client_clone = client.clone();

            // Maximize window to fill screen
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.maximize();
            }

            // Start notification subscription
            tauri::async_runtime::spawn(start_notification_subscription(
                handle.clone(),
                client_clone.clone(),
            ));

            // Start permission subscription
            tauri::async_runtime::spawn(start_permission_subscription(handle, client_clone));

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
