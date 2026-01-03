//! Error types for the notif.sh SDK.

use thiserror::Error;

/// Result type alias using NotifError.
pub type Result<T> = std::result::Result<T, NotifError>;

/// Errors that can occur when using the notif.sh SDK.
#[derive(Error, Debug)]
pub enum NotifError {
    /// Authentication error (missing or invalid API key).
    #[error("authentication error: {0}")]
    Auth(String),

    /// API error with HTTP status code.
    #[error("API error ({status}): {message}")]
    Api { status: u16, message: String },

    /// Connection error (network, WebSocket).
    #[error("connection error: {0}")]
    Connection(String),

    /// JSON serialization/deserialization error.
    #[error("serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    /// HTTP client error.
    #[error("HTTP error: {0}")]
    Http(#[from] reqwest::Error),

    /// WebSocket error.
    #[error("WebSocket error: {0}")]
    WebSocket(String),

    /// URL parsing error.
    #[error("invalid URL: {0}")]
    Url(#[from] url::ParseError),
}

impl NotifError {
    /// Create an authentication error.
    pub fn auth(msg: impl Into<String>) -> Self {
        Self::Auth(msg.into())
    }

    /// Create an API error with status code.
    pub fn api(status: u16, message: impl Into<String>) -> Self {
        Self::Api {
            status,
            message: message.into(),
        }
    }

    /// Create a connection error.
    pub fn connection(msg: impl Into<String>) -> Self {
        Self::Connection(msg.into())
    }

    /// Create a WebSocket error.
    pub fn websocket(msg: impl Into<String>) -> Self {
        Self::WebSocket(msg.into())
    }
}
