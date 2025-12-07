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

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Status {
    Pending,
    Delivered,
}

impl fmt::Display for Status {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Status::Pending => write!(f, "pending"),
            Status::Delivered => write!(f, "delivered"),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Notification {
    pub id: i64,
    pub message: String,
    pub priority: Priority,
    pub status: String,
    pub tags: Vec<String>,
    pub created_at: String,
    pub delivered_at: Option<String>,
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
