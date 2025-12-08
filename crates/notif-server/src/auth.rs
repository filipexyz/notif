use axum::{
    extract::Request,
    http::HeaderMap,
    middleware::Next,
    response::Response,
};
use std::sync::Arc;

use crate::error::AppError;

#[derive(Clone)]
pub struct ApiKeys(pub Arc<Vec<String>>);

pub async fn auth_middleware(
    headers: HeaderMap,
    request: Request,
    next: Next,
) -> Result<Response, AppError> {
    // Get API keys from request extensions
    let api_keys = request
        .extensions()
        .get::<ApiKeys>()
        .map(|k| k.0.clone())
        .unwrap_or_default();

    // If no keys configured, allow all requests (dev mode)
    if api_keys.is_empty() {
        return Ok(next.run(request).await);
    }

    // Check for API key header
    let provided_key = headers
        .get("x-api-key")
        .and_then(|v| v.to_str().ok());

    match provided_key {
        Some(key) if api_keys.iter().any(|k| k == key) => {
            Ok(next.run(request).await)
        }
        _ => Err(AppError::Unauthorized),
    }
}
