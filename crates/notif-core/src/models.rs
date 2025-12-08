use serde::{Deserialize, Serialize};
use std::fmt;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Priority {
    Low,
    Normal,
    High,
}

impl Default for Priority {
    fn default() -> Self {
        Priority::Normal
    }
}

impl fmt::Display for Priority {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Priority::Low => write!(f, "low"),
            Priority::Normal => write!(f, "normal"),
            Priority::High => write!(f, "high"),
        }
    }
}

impl std::str::FromStr for Priority {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "low" | "l" => Ok(Priority::Low),
            "normal" | "n" => Ok(Priority::Normal),
            "high" | "h" => Ok(Priority::High),
            _ => Err(format!("Invalid priority: {}. Use low, normal, or high", s)),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum Status {
    Pending,
    Approved,
    Dismissed,
    Delivered,
}

impl Default for Status {
    fn default() -> Self {
        Status::Pending
    }
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Status::Pending => write!(f, "pending"),
            Status::Approved => write!(f, "approved"),
            Status::Dismissed => write!(f, "dismissed"),
            Status::Delivered => write!(f, "delivered"),
        }
    }
}

impl std::str::FromStr for Status {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "pending" => Ok(Status::Pending),
            "approved" => Ok(Status::Approved),
            "dismissed" => Ok(Status::Dismissed),
            "delivered" => Ok(Status::Delivered),
            _ => Err(format!("Invalid status: {}", s)),
        }
    }
}

/// Source type for a notification
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum SourceType {
    #[default]
    Local,
    Remote,
}

impl fmt::Display for SourceType {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            SourceType::Local => write!(f, "local"),
            SourceType::Remote => write!(f, "remote"),
        }
    }
}

impl std::str::FromStr for SourceType {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s.to_lowercase().as_str() {
            "local" => Ok(SourceType::Local),
            "remote" => Ok(SourceType::Remote),
            _ => Err(format!("Invalid source type: {}", s)),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Notification {
    pub id: i64,
    pub message: String,
    pub priority: Priority,
    pub status: Status,
    pub tags: Vec<String>,
    pub created_at: String,
    pub delivered_at: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub content: Option<String>,
    #[serde(default)]
    pub source_type: SourceType,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_remote: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub source_id: Option<i64>,
}

impl Notification {
    /// Estimate token count for content (roughly 4 chars per token)
    pub fn content_tokens(&self) -> Option<usize> {
        self.content.as_ref().map(|c| (c.len() + 3) / 4)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum FilterMode {
    #[default]
    Include,
    Exclude,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TagFilter {
    pub tags: Vec<String>,
    #[serde(default)]
    pub mode: FilterMode,
}

impl TagFilter {
    pub fn matches(&self, notification_tags: &[String]) -> bool {
        // Tag-less notifications always show
        if notification_tags.is_empty() {
            return true;
        }

        let has_matching_tag = self
            .tags
            .iter()
            .any(|filter_tag| notification_tags.contains(filter_tag));

        match self.mode {
            FilterMode::Include => has_matching_tag,
            FilterMode::Exclude => !has_matching_tag,
        }
    }
}
