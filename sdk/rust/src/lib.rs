//! # notifsh
//!
//! Rust SDK for [notif.sh](https://notif.sh) - a managed pub/sub event hub.
//!
//! ## Quick Start
//!
//! ```no_run
//! use notifsh::Notif;
//! use serde_json::json;
//! use futures::StreamExt;
//!
//! #[tokio::main]
//! async fn main() -> notifsh::Result<()> {
//!     // Create client from environment
//!     let client = Notif::from_env()?;
//!
//!     // Emit an event
//!     let response = client.emit("orders.created", json!({"order_id": "123"})).await?;
//!     println!("Published: {}", response.id);
//!
//!     // Subscribe to events
//!     let mut stream = client.subscribe(&["orders.*"]).await?;
//!     while let Some(event) = stream.next().await {
//!         let event = event?;
//!         println!("{}: {:?}", event.topic, event.data);
//!     }
//!
//!     Ok(())
//! }
//! ```
//!
//! ## Configuration
//!
//! The client can be configured using environment variables or the builder pattern:
//!
//! ```no_run
//! use notifsh::Notif;
//! use std::time::Duration;
//!
//! // From environment (NOTIF_API_KEY)
//! let client = Notif::from_env()?;
//!
//! // Using builder
//! let client = Notif::builder("nsh_your_api_key")
//!     .server("http://localhost:8080")
//!     .timeout(Duration::from_secs(60))
//!     .build()?;
//! # Ok::<(), notifsh::NotifError>(())
//! ```
//!
//! ## Subscribe Options
//!
//! ```no_run
//! use notifsh::{Notif, SubscribeOptions};
//! use futures::StreamExt;
//!
//! # async fn example() -> notifsh::Result<()> {
//! let client = Notif::from_env()?;
//!
//! let mut stream = client
//!     .subscribe_with_options(
//!         &["orders.*"],
//!         SubscribeOptions::new()
//!             .auto_ack(false)      // Manual acknowledgment
//!             .from("beginning")    // Start from oldest events
//!             .group("workers"),    // Consumer group
//!     )
//!     .await?;
//!
//! while let Some(event) = stream.next().await {
//!     let event = event?;
//!     // Process event...
//!     event.ack().await?;
//!     // Or: event.nack(Some("5m")).await?;
//! }
//! # Ok(())
//! # }
//! ```

mod client;
mod error;
mod subscribe;
mod types;

pub use client::{Notif, NotifBuilder};
pub use error::{NotifError, Result};
pub use subscribe::EventStream;
pub use types::{
    CreateScheduleResponse, EmitResponse, Event, ListSchedulesResponse, RunScheduleResponse,
    Schedule, SubscribeOptions,
};
