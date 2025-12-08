use axum::{extract::State, Json};
use notif_core::{add_notification_with_content, Priority, Status};
use serde::{Deserialize, Serialize};

use crate::error::AppError;
use crate::server::AppState;

#[derive(Debug, Deserialize)]
pub struct CreateNotificationRequest {
    pub message: String,
    #[serde(default)]
    pub priority: Option<String>,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(default)]
    pub auto_approve: bool,
    #[serde(default)]
    pub content: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct CreateNotificationResponse {
    pub success: bool,
    pub data: Option<CreatedNotification>,
    pub error: Option<String>,
}

#[derive(Debug, Serialize)]
pub struct CreatedNotification {
    pub id: i64,
}

pub async fn create_notification(
    State(_state): State<AppState>,
    Json(payload): Json<CreateNotificationRequest>,
) -> Result<Json<CreateNotificationResponse>, AppError> {
    let priority = match payload.priority.as_deref() {
        Some("high") | Some("h") => Priority::High,
        Some("low") | Some("l") => Priority::Low,
        _ => Priority::Normal,
    };

    let status = if payload.auto_approve {
        Status::Approved
    } else {
        Status::Pending
    };

    let id = add_notification_with_content(
        &payload.message,
        priority,
        &payload.tags,
        status,
        payload.content.as_deref(),
    )
    .map_err(|e| AppError::Internal(e.to_string()))?;

    Ok(Json(CreateNotificationResponse {
        success: true,
        data: Some(CreatedNotification { id }),
        error: None,
    }))
}
