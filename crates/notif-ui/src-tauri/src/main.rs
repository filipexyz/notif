// Prevents additional console window on Windows in release
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use notif_core::{
    approve, approve_all_pending, dismiss, dismiss_all_pending, get_all_pending, init_db,
    update_message, Notification,
};

#[tauri::command]
fn get_pending() -> Result<Vec<Notification>, String> {
    init_db().map_err(|e| e.to_string())?;
    get_all_pending().map_err(|e| e.to_string())
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

fn main() {
    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .invoke_handler(tauri::generate_handler![
            get_pending,
            approve_notification,
            dismiss_notification,
            edit_notification,
            approve_all,
            dismiss_all,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
