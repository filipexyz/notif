use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

use crate::models::{Notification, Priority, Status};
use crate::remotes::RemoteConfig;

/// Response from the pull endpoint
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PullResponse {
    pub notifications: Vec<RemoteNotification>,
    pub last_id: i64,
}

/// A notification from a remote server
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RemoteNotification {
    pub id: i64,
    pub message: String,
    #[serde(default)]
    pub priority: Priority,
    #[serde(default)]
    pub status: Status,
    #[serde(default)]
    pub tags: Vec<String>,
    pub created_at: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<String>,
}

impl RemoteNotification {
    /// Convert to a full Notification with source info
    pub fn to_notification(&self, source_remote: &str) -> Notification {
        Notification {
            id: 0, // Will be assigned by local DB
            message: self.message.clone(),
            priority: self.priority,
            status: self.status,
            tags: self.tags.clone(),
            created_at: self.created_at.clone(),
            delivered_at: None,
            content: self.content.clone(),
            source_type: crate::models::SourceType::Remote,
            source_remote: Some(source_remote.to_string()),
            source_id: Some(self.id),
        }
    }
}

/// HTTP client for fetching notifications from remote servers
pub struct NotifClient {
    client: reqwest::Client,
}

impl NotifClient {
    pub fn new() -> Result<Self> {
        let client = reqwest::Client::builder()
            .timeout(std::time::Duration::from_secs(10))
            .build()
            .context("Failed to create HTTP client")?;

        Ok(Self { client })
    }

    /// Pull notifications from a remote server
    pub async fn pull(
        &self,
        remote: &RemoteConfig,
        since_id: i64,
    ) -> Result<PullResponse> {
        let url = format!("{}/notifications/pull", remote.url.trim_end_matches('/'));

        let mut request = self.client
            .get(&url)
            .header("X-API-Key", &remote.api_key)
            .query(&[("since_id", since_id.to_string())]);

        // Add tag filter if configured
        if !remote.tags.is_empty() {
            request = request.query(&[("tags", remote.tags.join(","))]);
        }

        let response = request
            .send()
            .await
            .with_context(|| format!("Failed to connect to {}", remote.name))?;

        if !response.status().is_success() {
            anyhow::bail!(
                "Remote {} returned status {}: {}",
                remote.name,
                response.status(),
                response.text().await.unwrap_or_default()
            );
        }

        let pull_response: PullResponse = response
            .json()
            .await
            .with_context(|| format!("Failed to parse response from {}", remote.name))?;

        Ok(pull_response)
    }

    /// Test connection to a remote server
    pub async fn test_connection(&self, remote: &RemoteConfig) -> Result<()> {
        let url = format!("{}/notifications", remote.url.trim_end_matches('/'));

        let response = self.client
            .get(&url)
            .header("X-API-Key", &remote.api_key)
            .query(&[("status", "pending"), ("limit", "1")])
            .send()
            .await
            .with_context(|| format!("Failed to connect to {}", remote.name))?;

        if response.status().is_success() {
            Ok(())
        } else if response.status().as_u16() == 401 {
            anyhow::bail!("Authentication failed - check API key")
        } else {
            anyhow::bail!("Server returned status {}", response.status())
        }
    }

    /// Forward a notification to a cascade target
    pub async fn forward_notification(
        &self,
        target_url: &str,
        api_key: &str,
        message: &str,
        priority: Priority,
        tags: &[String],
        content: Option<&str>,
    ) -> Result<i64> {
        let url = format!("{}/notifications", target_url.trim_end_matches('/'));

        #[derive(Serialize)]
        struct CreateRequest<'a> {
            message: &'a str,
            priority: Priority,
            tags: &'a [String],
            #[serde(skip_serializing_if = "Option::is_none")]
            content: Option<&'a str>,
        }

        let body = CreateRequest {
            message,
            priority,
            tags,
            content,
        };

        let response = self.client
            .post(&url)
            .header("X-API-Key", api_key)
            .header("Content-Type", "application/json")
            .json(&body)
            .send()
            .await
            .context("Failed to forward notification")?;

        if !response.status().is_success() {
            anyhow::bail!(
                "Forward failed with status {}: {}",
                response.status(),
                response.text().await.unwrap_or_default()
            );
        }

        #[derive(Deserialize)]
        struct CreateResponse {
            success: bool,
            data: Option<IdData>,
        }

        #[derive(Deserialize)]
        struct IdData {
            id: i64,
        }

        let result: CreateResponse = response.json().await?;
        if result.success {
            Ok(result.data.map(|d| d.id).unwrap_or(0))
        } else {
            anyhow::bail!("Forward failed - server reported failure")
        }
    }
}

impl Default for NotifClient {
    fn default() -> Self {
        Self::new().expect("Failed to create NotifClient")
    }
}
