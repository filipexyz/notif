use futures_util::StreamExt;
use notifsh::{Notif, SubscribeOptions};
use serde::Serialize;
use tauri::{AppHandle, Emitter};

/// Event for the frontend (simplified from notifsh::Event)
#[derive(Debug, Clone, Serialize)]
pub struct FrontendEvent {
    pub id: String,
    pub topic: String,
    pub data: serde_json::Value,
    pub timestamp: String,
}

/// Start the subscription in background
async fn start_subscription(app: AppHandle) -> Result<(), String> {
    println!("[event-log] Creating client...");
    let client = Notif::from_env().map_err(|e| {
        println!("[event-log] Client error: {}", e);
        e.to_string()
    })?;

    println!("[event-log] Subscribing to desktop.>...");
    // Subscribe to all desktop topics
    let mut stream = client
        .subscribe_with_options(
            &["desktop.>"],
            SubscribeOptions::new().auto_ack(true).from("latest"),
        )
        .await
        .map_err(|e| {
            println!("[event-log] Subscribe error: {}", e);
            e.to_string()
        })?;

    println!("[event-log] Subscribed! Waiting for frontend...");
    // Wait a moment for frontend to be ready, then emit connected status
    tokio::time::sleep(tokio::time::Duration::from_millis(500)).await;
    println!("[event-log] Emitting connected status...");
    let _ = app.emit("status", "connected");

    while let Some(result) = stream.next().await {
        match result {
            Ok(event) => {
                println!("[event-log] Received event: {} - {}", event.id, event.topic);
                let frontend_event = FrontendEvent {
                    id: event.id,
                    topic: event.topic,
                    data: event.data,
                    timestamp: event.timestamp.to_rfc3339(),
                };
                println!("[event-log] Emitting to frontend...");
                let _ = app.emit("event", &frontend_event);
            }
            Err(e) => {
                eprintln!("Subscription error: {}", e);
                let _ = app.emit("status", "error");
                break;
            }
        }
    }

    let _ = app.emit("status", "disconnected");
    Ok(())
}

#[tauri::command]
fn get_env_status() -> Result<String, String> {
    match std::env::var("NOTIF_API_KEY") {
        Ok(_) => Ok("configured".to_string()),
        Err(_) => Ok("not_configured".to_string()),
    }
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![get_env_status])
        .setup(|app| {
            let handle = app.handle().clone();

            // Start subscription in background
            tauri::async_runtime::spawn(async move {
                loop {
                    if let Err(e) = start_subscription(handle.clone()).await {
                        eprintln!("Subscription error: {}", e);
                    }
                    // Reconnect after delay
                    tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
                }
            });

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
