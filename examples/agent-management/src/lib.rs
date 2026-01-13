use futures_util::StreamExt;
use notifsh::{Notif, SubscribeOptions};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use tauri::{AppHandle, Emitter, Manager, State};
use tokio::sync::Mutex;

// ============== TYPES ==============

/// View mode for the overlay
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum ViewMode {
    #[default]
    Permissions,
    Agents,
    Chat,
}

/// Permission request from Claude Code
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PermissionRequest {
    pub id: Option<String>,
    pub tool_name: Option<String>,
    pub tool_input: Option<serde_json::Value>,
    pub session_id: Option<String>,
    pub cwd: Option<String>,
}

/// Session info for the UI
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionInfo {
    pub session_id: String,
    pub cwd: Option<String>,
    pub queue_count: usize,
}

/// Agent executor info
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentExecutor {
    pub kind: Option<String>,
    pub version: Option<String>,
    pub cli: Option<String>,
}

/// Agent project info
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentProject {
    pub name: Option<String>,
    pub path: Option<String>,
    pub repo: Option<String>,
}

/// Agent status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum AgentStatus {
    #[default]
    Idle,
    Busy,
    Offline,
}

/// Agent info from discovery
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Agent {
    pub name: String,
    pub description: Option<String>,
    pub hostname: Option<String>,
    pub tags: Option<Vec<String>>,
    pub executor: Option<AgentExecutor>,
    pub project: Option<AgentProject>,
    pub status: Option<AgentStatus>,
    #[serde(rename = "activeSessionId")]
    pub active_session_id: Option<String>,
}

/// Chat message for display
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessage {
    pub id: String,
    pub session_id: String,
    pub agent: String,
    pub content: String,
    pub is_user: bool,
    pub timestamp: String,
    pub kind: Option<String>,
    pub pr_url: Option<String>,
    pub cost_usd: Option<f64>,
}

/// Chat session
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatSession {
    pub session_id: String,
    pub agent: String,
    pub messages: Vec<ChatMessage>,
    pub status: String,
    pub created_at: String,
}

/// Badge counts for UI
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct BadgeCounts {
    pub permissions: usize,
    pub agents_busy: usize,
}

/// Agent event from subscription
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AgentEvent {
    #[serde(alias = "sessionId")]
    pub session_id: Option<String>,
    pub agent: Option<String>,
    pub kind: Option<String>,
    pub message: Option<String>,
    pub result: Option<String>,
    pub error: Option<serde_json::Value>,
    pub pr: Option<serde_json::Value>,
    #[serde(alias = "costUsd")]
    pub cost_usd: Option<f64>,
    pub timestamp: Option<String>,
}

// ============== STATE ==============

/// App state
struct AppState {
    client: Arc<Notif>,
    // Permissions
    queues: Mutex<HashMap<String, VecDeque<PermissionRequest>>>,
    active: Mutex<HashMap<String, PermissionRequest>>,
    // Agents
    agents: Mutex<HashMap<String, Agent>>,
    // Chat
    chat_sessions: Mutex<HashMap<String, ChatSession>>,
    active_chat_session: Mutex<Option<String>>,
    selected_agent: Mutex<Option<String>>,
    // UI
    current_view: Mutex<ViewMode>,
    window_visible: Mutex<bool>,
}

impl AppState {
    fn new(client: Arc<Notif>) -> Self {
        Self {
            client,
            queues: Mutex::new(HashMap::new()),
            active: Mutex::new(HashMap::new()),
            agents: Mutex::new(HashMap::new()),
            chat_sessions: Mutex::new(HashMap::new()),
            active_chat_session: Mutex::new(None),
            selected_agent: Mutex::new(None),
            current_view: Mutex::new(ViewMode::Permissions),
            window_visible: Mutex::new(false),
        }
    }
}

// ============== PERMISSION COMMANDS (existing) ==============

#[tauri::command]
async fn get_sessions(state: State<'_, Arc<AppState>>) -> Result<Vec<SessionInfo>, String> {
    let queues = state.queues.lock().await;
    let active = state.active.lock().await;

    let mut sessions: Vec<SessionInfo> = Vec::new();
    let mut session_ids: std::collections::HashSet<String> = queues.keys().cloned().collect();
    for id in active.keys() {
        session_ids.insert(id.clone());
    }

    for session_id in session_ids {
        let queue_count = queues.get(&session_id).map(|q| q.len()).unwrap_or(0);
        let has_active = active.contains_key(&session_id);
        let total = queue_count + if has_active { 1 } else { 0 };

        if total > 0 {
            let cwd = active
                .get(&session_id)
                .or_else(|| queues.get(&session_id).and_then(|q| q.front()))
                .and_then(|r| r.cwd.clone());

            sessions.push(SessionInfo {
                session_id,
                cwd,
                queue_count: total,
            });
        }
    }

    Ok(sessions)
}

#[tauri::command]
async fn get_current_permission(
    session_id: String,
    state: State<'_, Arc<AppState>>,
) -> Result<Option<PermissionRequest>, String> {
    let mut active = state.active.lock().await;

    if let Some(req) = active.get(&session_id) {
        return Ok(Some(req.clone()));
    }

    let mut queues = state.queues.lock().await;
    if let Some(queue) = queues.get_mut(&session_id) {
        if let Some(req) = queue.pop_front() {
            active.insert(session_id, req.clone());
            return Ok(Some(req));
        }
    }

    Ok(None)
}

#[tauri::command]
async fn respond_permission(
    session_id: String,
    decision: String,
    message: Option<String>,
    state: State<'_, Arc<AppState>>,
    app: AppHandle,
) -> Result<(), String> {
    {
        let mut active = state.active.lock().await;
        active.remove(&session_id);
    }

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

    state
        .client
        .emit("claude.permission.response", response)
        .await
        .map_err(|e| e.to_string())?;

    let _ = app.emit("permissions_updated", ());

    Ok(())
}

// ============== WINDOW COMMANDS ==============

#[tauri::command]
async fn show_window(state: State<'_, Arc<AppState>>, window: tauri::Window) -> Result<(), String> {
    *state.window_visible.lock().await = true;
    let _ = window.show();
    let _ = window.set_focus();
    Ok(())
}

#[tauri::command]
async fn hide_window(state: State<'_, Arc<AppState>>, window: tauri::Window) -> Result<(), String> {
    *state.window_visible.lock().await = false;
    let _ = window.hide();
    Ok(())
}

#[tauri::command]
async fn toggle_overlay(
    state: State<'_, Arc<AppState>>,
    window: tauri::Window,
) -> Result<bool, String> {
    let mut visible = state.window_visible.lock().await;
    *visible = !*visible;
    if *visible {
        let _ = window.show();
        let _ = window.set_focus();
    } else {
        let _ = window.hide();
    }
    Ok(*visible)
}

// ============== VIEW COMMANDS ==============

#[tauri::command]
async fn get_current_view(state: State<'_, Arc<AppState>>) -> Result<ViewMode, String> {
    Ok(*state.current_view.lock().await)
}

#[tauri::command]
async fn set_view(view: ViewMode, state: State<'_, Arc<AppState>>, app: AppHandle) -> Result<(), String> {
    *state.current_view.lock().await = view;
    let _ = app.emit("view_changed", view);
    Ok(())
}

#[tauri::command]
async fn get_badge_counts(state: State<'_, Arc<AppState>>) -> Result<BadgeCounts, String> {
    let queues = state.queues.lock().await;
    let active = state.active.lock().await;
    let agents = state.agents.lock().await;

    let permissions: usize = queues.values().map(|q| q.len()).sum::<usize>() + active.len();
    let agents_busy = agents
        .values()
        .filter(|a| a.status == Some(AgentStatus::Busy))
        .count();

    Ok(BadgeCounts {
        permissions,
        agents_busy,
    })
}

// ============== AGENT COMMANDS ==============

#[tauri::command]
async fn discover_agents(state: State<'_, Arc<AppState>>) -> Result<(), String> {
    state
        .client
        .emit("agents.discover", json!({}))
        .await
        .map_err(|e| e.to_string())?;
    Ok(())
}

#[tauri::command]
async fn get_agents(state: State<'_, Arc<AppState>>) -> Result<Vec<Agent>, String> {
    let agents = state.agents.lock().await;
    Ok(agents.values().cloned().collect())
}

#[tauri::command]
async fn select_agent(
    agent_name: String,
    state: State<'_, Arc<AppState>>,
    app: AppHandle,
) -> Result<(), String> {
    *state.selected_agent.lock().await = Some(agent_name.clone());
    let _ = app.emit("agent_selected", &agent_name);
    Ok(())
}

#[tauri::command]
async fn get_selected_agent(state: State<'_, Arc<AppState>>) -> Result<Option<String>, String> {
    Ok(state.selected_agent.lock().await.clone())
}

// ============== CHAT COMMANDS ==============

#[tauri::command]
async fn send_prompt(
    agent_name: String,
    prompt: String,
    state: State<'_, Arc<AppState>>,
    app: AppHandle,
) -> Result<String, String> {
    let session_id = format!("sess_{}", &uuid::Uuid::new_v4().to_string()[..12]);

    let message = json!({
        "session_id": session_id,
        "agent": agent_name,
        "kind": "prompt",
        "prompt": prompt,
    });

    let topic = format!("agents.{}.session.create", agent_name);
    state
        .client
        .emit(&topic, message)
        .await
        .map_err(|e| e.to_string())?;

    let chat_session = ChatSession {
        session_id: session_id.clone(),
        agent: agent_name.clone(),
        messages: vec![ChatMessage {
            id: uuid::Uuid::new_v4().to_string(),
            session_id: session_id.clone(),
            agent: agent_name.clone(),
            content: prompt,
            is_user: true,
            timestamp: chrono::Utc::now().to_rfc3339(),
            kind: None,
            pr_url: None,
            cost_usd: None,
        }],
        status: "running".to_string(),
        created_at: chrono::Utc::now().to_rfc3339(),
    };

    {
        let mut sessions = state.chat_sessions.lock().await;
        sessions.insert(session_id.clone(), chat_session);
    }
    *state.active_chat_session.lock().await = Some(session_id.clone());

    let _ = app.emit("chat_session_created", &session_id);
    Ok(session_id)
}

#[tauri::command]
async fn send_followup(
    session_id: String,
    message: String,
    state: State<'_, Arc<AppState>>,
    app: AppHandle,
) -> Result<(), String> {
    let agent_name = {
        let sessions = state.chat_sessions.lock().await;
        sessions
            .get(&session_id)
            .map(|s| s.agent.clone())
            .ok_or("Session not found")?
    };

    let msg = json!({
        "session_id": session_id,
        "agent": agent_name,
        "kind": "resume",
        "prompt": message,
    });

    let topic = format!("agents.{}.session.message", agent_name);
    state
        .client
        .emit(&topic, msg)
        .await
        .map_err(|e| e.to_string())?;

    {
        let mut sessions = state.chat_sessions.lock().await;
        if let Some(session) = sessions.get_mut(&session_id) {
            session.messages.push(ChatMessage {
                id: uuid::Uuid::new_v4().to_string(),
                session_id: session_id.clone(),
                agent: agent_name,
                content: message,
                is_user: true,
                timestamp: chrono::Utc::now().to_rfc3339(),
                kind: None,
                pr_url: None,
                cost_usd: None,
            });
            session.status = "running".to_string();
        }
    }

    let _ = app.emit("chat_message_sent", &session_id);
    Ok(())
}

#[tauri::command]
async fn cancel_session(session_id: String, state: State<'_, Arc<AppState>>) -> Result<(), String> {
    let agent_name = {
        let sessions = state.chat_sessions.lock().await;
        sessions
            .get(&session_id)
            .map(|s| s.agent.clone())
            .ok_or("Session not found")?
    };

    let msg = json!({
        "session_id": session_id,
        "agent": agent_name,
        "kind": "cancel",
    });

    let topic = format!("agents.{}.session.cancel", agent_name);
    state
        .client
        .emit(&topic, msg)
        .await
        .map_err(|e| e.to_string())?;

    Ok(())
}

#[tauri::command]
async fn get_chat_sessions(state: State<'_, Arc<AppState>>) -> Result<Vec<ChatSession>, String> {
    let sessions = state.chat_sessions.lock().await;
    Ok(sessions.values().cloned().collect())
}

#[tauri::command]
async fn get_active_chat_session(
    state: State<'_, Arc<AppState>>,
) -> Result<Option<ChatSession>, String> {
    let active_id = state.active_chat_session.lock().await;
    if let Some(id) = active_id.as_ref() {
        let sessions = state.chat_sessions.lock().await;
        Ok(sessions.get(id).cloned())
    } else {
        Ok(None)
    }
}

#[tauri::command]
async fn set_active_chat_session(
    session_id: Option<String>,
    state: State<'_, Arc<AppState>>,
) -> Result<(), String> {
    *state.active_chat_session.lock().await = session_id;
    Ok(())
}

// ============== SUBSCRIPTIONS ==============

async fn start_permission_subscription(app: AppHandle, state: Arc<AppState>) {
    loop {
        match state
            .client
            .subscribe_with_options(
                &["claude.permission.request"],
                SubscribeOptions::new().auto_ack(true).from("latest"),
            )
            .await
        {
            Ok(mut stream) => {
                while let Some(result) = stream.next().await {
                    if let Ok(event) = result {
                        if let Ok(mut request) =
                            serde_json::from_value::<PermissionRequest>(event.data)
                        {
                            if request.id.is_none() {
                                request.id = Some(event.id.clone());
                            }

                            let session_id = request
                                .session_id
                                .clone()
                                .unwrap_or_else(|| "default".to_string());

                            {
                                let mut queues = state.queues.lock().await;
                                queues
                                    .entry(session_id.clone())
                                    .or_insert_with(VecDeque::new)
                                    .push_back(request);
                            }

                            let _ = app.emit("permissions_updated", ());
                            let _ = app.emit("permission_request", &session_id);
                        }
                    }
                }
            }
            Err(e) => eprintln!("Permission subscription error: {}", e),
        }
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

async fn start_agent_subscription(app: AppHandle, state: Arc<AppState>) {
    loop {
        match state
            .client
            .subscribe_with_options(
                &["agents.available"],
                SubscribeOptions::new().auto_ack(true).from("latest"),
            )
            .await
        {
            Ok(mut stream) => {
                while let Some(result) = stream.next().await {
                    if let Ok(event) = result {
                        if let Ok(agent) = serde_json::from_value::<Agent>(event.data) {
                            let agent_name = agent.name.clone();
                            {
                                let mut agents = state.agents.lock().await;
                                agents.insert(agent_name.clone(), agent);
                            }
                            let _ = app.emit("agent_discovered", &agent_name);
                        }
                    }
                }
            }
            Err(e) => eprintln!("Agent subscription error: {}", e),
        }
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

async fn start_session_subscription(app: AppHandle, state: Arc<AppState>) {
    let topics = [
        "agents.*.session.started",
        "agents.*.session.output",
        "agents.*.session.completed",
        "agents.*.session.failed",
        "agents.*.session.blocked",
    ];

    loop {
        match state
            .client
            .subscribe_with_options(
                &topics,
                SubscribeOptions::new().auto_ack(true).from("latest"),
            )
            .await
        {
            Ok(mut stream) => {
                while let Some(result) = stream.next().await {
                    if let Ok(event) = result {
                        if let Ok(agent_event) =
                            serde_json::from_value::<AgentEvent>(event.data.clone())
                        {
                            handle_agent_event(&app, &state, agent_event, &event.topic).await;
                        }
                    }
                }
            }
            Err(e) => eprintln!("Session subscription error: {}", e),
        }
        tokio::time::sleep(tokio::time::Duration::from_secs(5)).await;
    }
}

async fn handle_agent_event(app: &AppHandle, state: &Arc<AppState>, event: AgentEvent, _topic: &str) {
    let session_id = match &event.session_id {
        Some(id) => id.clone(),
        None => return,
    };

    let agent_name = event.agent.clone().unwrap_or_default();
    let kind = event.kind.clone().unwrap_or_default();

    let chat_message = ChatMessage {
        id: uuid::Uuid::new_v4().to_string(),
        session_id: session_id.clone(),
        agent: agent_name.clone(),
        content: event
            .message
            .clone()
            .or(event.result.clone())
            .unwrap_or_default(),
        is_user: false,
        timestamp: event
            .timestamp
            .clone()
            .unwrap_or_else(|| chrono::Utc::now().to_rfc3339()),
        kind: Some(kind.clone()),
        pr_url: event
            .pr
            .as_ref()
            .and_then(|p| p.get("url").and_then(|u| u.as_str()).map(|s| s.to_string())),
        cost_usd: event.cost_usd,
    };

    {
        let mut sessions = state.chat_sessions.lock().await;
        if let Some(session) = sessions.get_mut(&session_id) {
            session.messages.push(chat_message.clone());

            session.status = match kind.as_str() {
                "started" | "progress" | "output" => "running".to_string(),
                "completed" => "completed".to_string(),
                "failed" => "failed".to_string(),
                "blocked" => "blocked".to_string(),
                _ => session.status.clone(),
            };
        }
    }

    // Update agent status
    if !agent_name.is_empty() {
        let mut agents = state.agents.lock().await;
        if let Some(agent) = agents.get_mut(&agent_name) {
            agent.status = match kind.as_str() {
                "started" | "progress" | "output" | "blocked" => Some(AgentStatus::Busy),
                "completed" | "failed" => Some(AgentStatus::Idle),
                _ => agent.status,
            };
            agent.active_session_id = match kind.as_str() {
                "completed" | "failed" => None,
                _ => Some(session_id.clone()),
            };
        }
    }

    let _ = app.emit("agent_event", &event);
    let _ = app.emit("chat_message_received", &chat_message);
}

// ============== MAIN ==============

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    let client = Arc::new(Notif::from_env().expect("NOTIF_API_KEY must be set"));
    let state = Arc::new(AppState::new(client));

    tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .manage(state.clone())
        .invoke_handler(tauri::generate_handler![
            // Permissions
            get_sessions,
            get_current_permission,
            respond_permission,
            // Window
            show_window,
            hide_window,
            toggle_overlay,
            // View
            get_current_view,
            set_view,
            get_badge_counts,
            // Agents
            discover_agents,
            get_agents,
            select_agent,
            get_selected_agent,
            // Chat
            send_prompt,
            send_followup,
            cancel_session,
            get_chat_sessions,
            get_active_chat_session,
            set_active_chat_session,
        ])
        .setup(move |app| {
            let handle = app.handle().clone();
            let state_clone = state.clone();

            // Maximize window to fill screen
            if let Some(window) = app.get_webview_window("main") {
                let _ = window.maximize();
            }

            // Register global shortcuts
            #[cfg(desktop)]
            {
                use tauri_plugin_global_shortcut::{GlobalShortcutExt, Shortcut};

                let shortcut: Shortcut = "CmdOrCtrl+Shift+Space".parse().unwrap();
                let handle_for_shortcut = handle.clone();

                app.handle().plugin(
                    tauri_plugin_global_shortcut::Builder::new()
                        .with_handler(move |_app, _shortcut, event| {
                            if event.state() == tauri_plugin_global_shortcut::ShortcutState::Pressed
                            {
                                let _ = handle_for_shortcut.emit("toggle_overlay_shortcut", ());
                            }
                        })
                        .build(),
                )?;

                // Register the shortcut
                if let Err(e) = app.global_shortcut().register(shortcut) {
                    eprintln!("Failed to register global shortcut: {}", e);
                }
            }

            // Start subscriptions
            tauri::async_runtime::spawn(start_permission_subscription(
                handle.clone(),
                state_clone.clone(),
            ));
            tauri::async_runtime::spawn(start_agent_subscription(
                handle.clone(),
                state_clone.clone(),
            ));
            tauri::async_runtime::spawn(start_session_subscription(handle, state_clone));

            Ok(())
        })
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
