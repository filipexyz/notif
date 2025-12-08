use axum::{
    extract::{Path, Query, State},
    Json,
};
use notif_core::{
    approve, approve_all_pending, delete_notification, dismiss, dismiss_all_pending,
    get_all_notifications, get_by_status, Notification, Status,
};
use serde::{Deserialize, Serialize};

use crate::error::AppError;
use crate::server::AppState;

#[derive(Debug, Deserialize)]
pub struct ListQuery {
    pub status: Option<String>,
    #[serde(default = "default_limit")]
    pub limit: usize,
}

fn default_limit() -> usize {
    100
}

#[derive(Debug, Serialize)]
pub struct ApiResponse<T> {
    pub success: bool,
    pub data: Option<T>,
    pub error: Option<String>,
}

impl<T> ApiResponse<T> {
    pub fn ok(data: T) -> Self {
        Self {
            success: true,
            data: Some(data),
            error: None,
        }
    }
}

#[derive(Debug, Serialize)]
pub struct CountResponse {
    pub count: usize,
}

// GET /notifications
pub async fn list_notifications(
    State(_state): State<AppState>,
    Query(query): Query<ListQuery>,
) -> Result<Json<ApiResponse<Vec<Notification>>>, AppError> {
    let notifications = match query.status.as_deref() {
        Some("pending") => get_by_status(Status::Pending, query.limit),
        Some("approved") => get_by_status(Status::Approved, query.limit),
        Some("dismissed") => get_by_status(Status::Dismissed, query.limit),
        Some("delivered") => get_by_status(Status::Delivered, query.limit),
        _ => get_all_notifications(query.limit),
    }
    .map_err(|e| AppError::Internal(e.to_string()))?;

    Ok(Json(ApiResponse::ok(notifications)))
}

// GET /notifications/:id
pub async fn get_notification(
    State(_state): State<AppState>,
    Path(id): Path<i64>,
) -> Result<Json<ApiResponse<Notification>>, AppError> {
    // Get all and find by id (simple approach)
    let notifications = get_all_notifications(1000)
        .map_err(|e| AppError::Internal(e.to_string()))?;

    let notification = notifications
        .into_iter()
        .find(|n| n.id == id)
        .ok_or(AppError::NotFound)?;

    Ok(Json(ApiResponse::ok(notification)))
}

// PUT /notifications/:id/approve
pub async fn approve_notification(
    State(_state): State<AppState>,
    Path(id): Path<i64>,
) -> Result<Json<ApiResponse<()>>, AppError> {
    approve(id).map_err(|e| AppError::Internal(e.to_string()))?;
    Ok(Json(ApiResponse::ok(())))
}

// PUT /notifications/:id/dismiss
pub async fn dismiss_notification(
    State(_state): State<AppState>,
    Path(id): Path<i64>,
) -> Result<Json<ApiResponse<()>>, AppError> {
    dismiss(id).map_err(|e| AppError::Internal(e.to_string()))?;
    Ok(Json(ApiResponse::ok(())))
}

// DELETE /notifications/:id
pub async fn delete_notif(
    State(_state): State<AppState>,
    Path(id): Path<i64>,
) -> Result<Json<ApiResponse<()>>, AppError> {
    delete_notification(id).map_err(|e| AppError::Internal(e.to_string()))?;
    Ok(Json(ApiResponse::ok(())))
}

// POST /notifications/approve-all
pub async fn approve_all(
    State(_state): State<AppState>,
) -> Result<Json<ApiResponse<CountResponse>>, AppError> {
    let count = approve_all_pending().map_err(|e| AppError::Internal(e.to_string()))?;
    Ok(Json(ApiResponse::ok(CountResponse { count })))
}

// POST /notifications/dismiss-all
pub async fn dismiss_all(
    State(_state): State<AppState>,
) -> Result<Json<ApiResponse<CountResponse>>, AppError> {
    let count = dismiss_all_pending().map_err(|e| AppError::Internal(e.to_string()))?;
    Ok(Json(ApiResponse::ok(CountResponse { count })))
}
