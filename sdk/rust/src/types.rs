//! Data types for the notif.sh SDK.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use tokio::sync::mpsc;

use crate::error::Result;

/// Response from emitting an event.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[non_exhaustive]
pub struct EmitResponse {
    /// Event ID.
    pub id: String,
    /// Topic the event was published to.
    pub topic: String,
    /// When the event was created.
    #[serde(rename = "created_at")]
    pub created_at: DateTime<Utc>,
}

/// Options for subscribing to topics.
#[derive(Debug, Clone)]
pub struct SubscribeOptions {
    /// Automatically acknowledge events (default: true).
    pub auto_ack: bool,
    /// Starting position: "latest", "beginning", or ISO8601 timestamp.
    pub from: Option<String>,
    /// Consumer group name for load balancing.
    pub group: Option<String>,
}

impl Default for SubscribeOptions {
    fn default() -> Self {
        Self::new()
    }
}

impl SubscribeOptions {
    /// Create new subscribe options with defaults (auto_ack: true).
    pub fn new() -> Self {
        Self {
            auto_ack: true,
            from: None,
            group: None,
        }
    }

    /// Set auto_ack option.
    pub fn auto_ack(mut self, auto_ack: bool) -> Self {
        self.auto_ack = auto_ack;
        self
    }

    /// Set starting position.
    pub fn from(mut self, from: impl Into<String>) -> Self {
        self.from = Some(from.into());
        self
    }

    /// Set consumer group.
    pub fn group(mut self, group: impl Into<String>) -> Self {
        self.group = Some(group.into());
        self
    }
}

/// An event received from a subscription.
#[derive(Debug, Clone)]
#[non_exhaustive]
pub struct Event {
    /// Event ID.
    pub id: String,
    /// Topic the event was received from.
    pub topic: String,
    /// Event payload.
    pub data: serde_json::Value,
    /// When the event was created.
    pub timestamp: DateTime<Utc>,
    /// Current delivery attempt number.
    pub attempt: u32,
    /// Maximum delivery attempts before DLQ.
    pub max_attempts: u32,
    /// Internal sender for ack/nack (None if auto_ack is true).
    pub(crate) ack_tx: Option<mpsc::Sender<AckMessage>>,
}

impl Event {
    /// Acknowledge the event.
    ///
    /// This is a no-op if auto_ack is enabled.
    pub async fn ack(&self) -> Result<()> {
        if let Some(tx) = &self.ack_tx {
            let _ = tx
                .send(AckMessage::Ack {
                    id: self.id.clone(),
                })
                .await;
        }
        Ok(())
    }

    /// Negatively acknowledge the event.
    ///
    /// The event will be redelivered after the specified delay.
    /// Default delay is "5m" (5 minutes).
    ///
    /// This is a no-op if auto_ack is enabled.
    pub async fn nack(&self, retry_in: Option<&str>) -> Result<()> {
        if let Some(tx) = &self.ack_tx {
            let _ = tx
                .send(AckMessage::Nack {
                    id: self.id.clone(),
                    retry_in: retry_in.map(String::from),
                })
                .await;
        }
        Ok(())
    }
}

/// Internal message for ack/nack operations.
#[derive(Debug)]
pub(crate) enum AckMessage {
    Ack { id: String },
    Nack { id: String, retry_in: Option<String> },
}

// WebSocket protocol messages

#[derive(Debug, Serialize)]
pub(crate) struct SubscribeMessage {
    pub action: String,
    pub topics: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub options: Option<SubscribeOptionsWire>,
}

#[derive(Debug, Serialize)]
pub(crate) struct SubscribeOptionsWire {
    pub auto_ack: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub from: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub group: Option<String>,
}

#[derive(Debug, Serialize)]
pub(crate) struct AckWireMessage {
    pub action: String,
    pub id: String,
}

#[derive(Debug, Serialize)]
pub(crate) struct NackWireMessage {
    pub action: String,
    pub id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub retry_in: Option<String>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ServerMessage {
    #[serde(rename = "type")]
    pub msg_type: String,
    // Event fields
    pub id: Option<String>,
    pub topic: Option<String>,
    pub data: Option<serde_json::Value>,
    pub timestamp: Option<DateTime<Utc>>,
    pub attempt: Option<u32>,
    pub max_attempts: Option<u32>,
    // Subscribed fields
    pub topics: Option<Vec<String>>,
    pub consumer_id: Option<String>,
    // Error fields
    pub code: Option<String>,
    pub message: Option<String>,
}

// HTTP API types

#[derive(Debug, Serialize)]
pub(crate) struct EmitRequest<'a, T: Serialize> {
    pub topic: &'a str,
    pub data: T,
}

// Schedule types

/// Response from creating a scheduled event.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[non_exhaustive]
pub struct CreateScheduleResponse {
    /// Schedule ID.
    pub id: String,
    /// Topic the event will be published to.
    pub topic: String,
    /// When the event is scheduled to be emitted.
    pub scheduled_for: DateTime<Utc>,
    /// When the schedule was created.
    pub created_at: DateTime<Utc>,
}

/// A scheduled event.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[non_exhaustive]
pub struct Schedule {
    /// Schedule ID.
    pub id: String,
    /// Topic the event will be published to.
    pub topic: String,
    /// Event payload.
    pub data: serde_json::Value,
    /// When the event is scheduled to be emitted.
    pub scheduled_for: DateTime<Utc>,
    /// Status: pending, completed, cancelled, failed.
    pub status: String,
    /// Error message if failed.
    pub error: Option<String>,
    /// When the schedule was created.
    pub created_at: DateTime<Utc>,
    /// When the event was executed (if completed).
    pub executed_at: Option<DateTime<Utc>>,
}

/// Response from listing scheduled events.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[non_exhaustive]
pub struct ListSchedulesResponse {
    /// List of schedules.
    pub schedules: Vec<Schedule>,
    /// Total count (for pagination).
    pub total: i64,
}

/// Response from running a scheduled event immediately.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[non_exhaustive]
pub struct RunScheduleResponse {
    /// Schedule ID.
    pub schedule_id: String,
    /// Emitted event ID.
    pub event_id: String,
}

#[derive(Debug, Serialize)]
pub(crate) struct CreateScheduleRequest<'a, T: Serialize> {
    pub topic: &'a str,
    pub data: T,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub scheduled_for: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none", rename = "in")]
    pub in_duration: Option<&'a str>,
}
