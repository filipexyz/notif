//! Notif client implementation.

use std::env;
use std::sync::Arc;
use std::time::Duration;

use reqwest::Client as HttpClient;
use serde::Serialize;

use crate::error::{NotifError, Result};
use crate::subscribe::EventStream;
use crate::types::{EmitRequest, EmitResponse, SubscribeOptions};

const DEFAULT_SERVER: &str = "https://api.notif.sh";
const DEFAULT_TIMEOUT_SECS: u64 = 30;
const API_KEY_PREFIX: &str = "nsh_";
const ENV_VAR_NAME: &str = "NOTIF_API_KEY";

/// Builder for creating a Notif client with custom options.
#[derive(Debug, Clone)]
pub struct NotifBuilder {
    api_key: String,
    server: String,
    timeout: Duration,
}

impl NotifBuilder {
    /// Create a new builder with the given API key.
    pub fn new(api_key: impl Into<String>) -> Self {
        Self {
            api_key: api_key.into(),
            server: DEFAULT_SERVER.to_string(),
            timeout: Duration::from_secs(DEFAULT_TIMEOUT_SECS),
        }
    }

    /// Set the server URL.
    pub fn server(mut self, server: impl Into<String>) -> Self {
        self.server = server.into();
        self
    }

    /// Set the request timeout.
    pub fn timeout(mut self, timeout: Duration) -> Self {
        self.timeout = timeout;
        self
    }

    /// Build the Notif client.
    pub fn build(self) -> Result<Notif> {
        // Validate API key
        if !self.api_key.starts_with(API_KEY_PREFIX) {
            return Err(NotifError::auth(format!(
                "API key must start with '{}'",
                API_KEY_PREFIX
            )));
        }

        let http_client = HttpClient::builder()
            .timeout(self.timeout)
            .build()
            .map_err(|e| NotifError::connection(e.to_string()))?;

        Ok(Notif {
            inner: Arc::new(NotifInner {
                api_key: self.api_key,
                server: self.server,
                http_client,
                timeout: self.timeout,
            }),
        })
    }
}

/// Internal shared state for the client.
pub(crate) struct NotifInner {
    pub(crate) api_key: String,
    pub(crate) server: String,
    pub(crate) http_client: HttpClient,
    pub(crate) timeout: Duration,
}

/// The notif.sh client.
///
/// # Example
///
/// ```no_run
/// use notifsh::Notif;
/// use serde_json::json;
///
/// #[tokio::main]
/// async fn main() -> notifsh::Result<()> {
///     let client = Notif::from_env()?;
///
///     // Emit an event
///     let response = client.emit("orders.created", json!({"order_id": "123"})).await?;
///     println!("Event ID: {}", response.id);
///
///     Ok(())
/// }
/// ```
#[derive(Clone)]
pub struct Notif {
    pub(crate) inner: Arc<NotifInner>,
}

impl Notif {
    /// Create a new client from environment variables.
    ///
    /// Reads the API key from the `NOTIF_API_KEY` environment variable.
    pub fn from_env() -> Result<Self> {
        let api_key = env::var(ENV_VAR_NAME)
            .map_err(|_| NotifError::auth(format!("{} environment variable not set", ENV_VAR_NAME)))?;

        NotifBuilder::new(api_key).build()
    }

    /// Create a new builder with the given API key.
    pub fn builder(api_key: impl Into<String>) -> NotifBuilder {
        NotifBuilder::new(api_key)
    }

    /// Get the configured server URL.
    pub fn server_url(&self) -> &str {
        &self.inner.server
    }

    /// Emit an event to a topic.
    ///
    /// # Arguments
    ///
    /// * `topic` - The topic to publish to (e.g., "orders.created")
    /// * `data` - The event payload (any serializable type)
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use notifsh::Notif;
    /// # use serde_json::json;
    /// # async fn example() -> notifsh::Result<()> {
    /// let client = Notif::from_env()?;
    ///
    /// // Using json! macro
    /// client.emit("orders.created", json!({"order_id": "123"})).await?;
    ///
    /// // Or using a struct
    /// #[derive(serde::Serialize)]
    /// struct Order { order_id: String }
    /// client.emit("orders.created", Order { order_id: "123".into() }).await?;
    /// # Ok(())
    /// # }
    /// ```
    pub async fn emit<T: Serialize>(
        &self,
        topic: &str,
        data: T,
    ) -> Result<EmitResponse> {
        let url = format!("{}/api/v1/emit", self.inner.server);

        let request = EmitRequest { topic, data };

        let response = self
            .inner
            .http_client
            .post(&url)
            .bearer_auth(&self.inner.api_key)
            .json(&request)
            .send()
            .await?;

        let status = response.status();
        if !status.is_success() {
            let message = response.text().await.unwrap_or_default();
            if status.as_u16() == 401 {
                return Err(NotifError::auth(message));
            }
            return Err(NotifError::api(status.as_u16(), message));
        }

        let emit_response: EmitResponse = response.json().await?;
        Ok(emit_response)
    }

    /// Subscribe to one or more topics.
    ///
    /// Returns an async stream of events. Use with `futures::StreamExt`.
    ///
    /// # Arguments
    ///
    /// * `topics` - Topics to subscribe to (supports wildcards like "orders.*")
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use notifsh::Notif;
    /// # use futures::StreamExt;
    /// # async fn example() -> notifsh::Result<()> {
    /// let client = Notif::from_env()?;
    /// let mut stream = client.subscribe(&["orders.*"]).await?;
    ///
    /// while let Some(event) = stream.next().await {
    ///     let event = event?;
    ///     println!("Got event: {} - {:?}", event.topic, event.data);
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn subscribe(&self, topics: &[&str]) -> Result<EventStream> {
        self.subscribe_with_options(topics, SubscribeOptions::new()).await
    }

    /// Subscribe to topics with custom options.
    ///
    /// # Arguments
    ///
    /// * `topics` - Topics to subscribe to
    /// * `options` - Subscription options (auto_ack, from, group)
    ///
    /// # Example
    ///
    /// ```no_run
    /// # use notifsh::{Notif, SubscribeOptions};
    /// # use futures::StreamExt;
    /// # async fn example() -> notifsh::Result<()> {
    /// let client = Notif::from_env()?;
    /// let mut stream = client
    ///     .subscribe_with_options(
    ///         &["orders.*"],
    ///         SubscribeOptions::new()
    ///             .auto_ack(false)
    ///             .from("beginning")
    ///             .group("worker-pool"),
    ///     )
    ///     .await?;
    ///
    /// while let Some(event) = stream.next().await {
    ///     let event = event?;
    ///     // Process event...
    ///     event.ack().await?;
    /// }
    /// # Ok(())
    /// # }
    /// ```
    pub async fn subscribe_with_options(
        &self,
        topics: &[&str],
        options: SubscribeOptions,
    ) -> Result<EventStream> {
        EventStream::connect(self.inner.clone(), topics, options).await
    }
}
