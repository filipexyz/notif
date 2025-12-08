use anyhow::{Context, Result};
use directories::ProjectDirs;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

const REMOTES_FILE: &str = "remotes.toml";

/// Mode for handling remote notifications
#[derive(Debug, Clone, Copy, PartialEq, Eq, Default, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RemoteMode {
    /// Store notifications locally in DB before outputting
    #[default]
    Store,
    /// Pass through directly to hook output (don't persist)
    Passthrough,
}

/// Configuration for a remote notif server
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RemoteConfig {
    pub name: String,
    pub url: String,
    pub api_key: String,
    #[serde(default)]
    pub mode: RemoteMode,
    #[serde(default)]
    pub tags: Vec<String>,
    #[serde(default)]
    pub auto_approve: bool,
}

/// Cascade rule for forwarding notifications
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CascadeRule {
    pub name: String,
    #[serde(default)]
    pub match_tags: Vec<String>,
    pub target_url: String,
    pub target_api_key: String,
    #[serde(default)]
    pub add_tags: Vec<String>,
    #[serde(default)]
    pub remove_tags: Vec<String>,
}

/// Global settings for remote polling
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct RemoteSettings {
    #[serde(default = "default_poll_interval")]
    pub poll_interval: u64,
}

fn default_poll_interval() -> u64 {
    60
}

/// Full remotes configuration from ~/.notif/remotes.toml
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
pub struct RemotesConfig {
    #[serde(default)]
    pub settings: RemoteSettings,
    #[serde(default, rename = "remotes")]
    pub remotes: Vec<RemoteConfig>,
    #[serde(default, rename = "cascade")]
    pub cascade_rules: Vec<CascadeRule>,
}

impl RemotesConfig {
    /// Get a remote by name
    pub fn get_remote(&self, name: &str) -> Option<&RemoteConfig> {
        self.remotes.iter().find(|r| r.name == name)
    }

    /// Get all remotes with a specific mode
    pub fn remotes_by_mode(&self, mode: RemoteMode) -> Vec<&RemoteConfig> {
        self.remotes.iter().filter(|r| r.mode == mode).collect()
    }
}

/// Get the path to ~/.notif/ directory
pub fn get_notif_dir() -> Result<PathBuf> {
    let proj_dirs = ProjectDirs::from("com", "filipelabs", "notif")
        .context("Could not determine config directory")?;

    let config_dir = proj_dirs.config_dir();
    fs::create_dir_all(config_dir).context("Could not create config directory")?;

    Ok(config_dir.to_path_buf())
}

/// Get the path to remotes.toml
pub fn get_remotes_path() -> Result<PathBuf> {
    Ok(get_notif_dir()?.join(REMOTES_FILE))
}

/// Load remotes configuration from ~/.notif/remotes.toml
pub fn load_remotes_config() -> Result<RemotesConfig> {
    let path = get_remotes_path()?;

    if !path.exists() {
        return Ok(RemotesConfig::default());
    }

    let content = fs::read_to_string(&path)
        .with_context(|| format!("Could not read {:?}", path))?;

    let config: RemotesConfig = toml::from_str(&content)
        .with_context(|| format!("Invalid TOML in {:?}", path))?;

    Ok(config)
}

/// Save remotes configuration to ~/.notif/remotes.toml
pub fn save_remotes_config(config: &RemotesConfig) -> Result<PathBuf> {
    let path = get_remotes_path()?;
    let content = toml::to_string_pretty(config)?;
    fs::write(&path, content)?;
    Ok(path)
}

/// Add a new remote to the configuration
pub fn add_remote(remote: RemoteConfig) -> Result<()> {
    let mut config = load_remotes_config()?;

    // Remove existing remote with same name
    config.remotes.retain(|r| r.name != remote.name);
    config.remotes.push(remote);

    save_remotes_config(&config)?;
    Ok(())
}

/// Remove a remote from the configuration
pub fn remove_remote(name: &str) -> Result<bool> {
    let mut config = load_remotes_config()?;
    let initial_len = config.remotes.len();
    config.remotes.retain(|r| r.name != name);

    if config.remotes.len() < initial_len {
        save_remotes_config(&config)?;
        Ok(true)
    } else {
        Ok(false)
    }
}

/// Add a cascade rule to the configuration
pub fn add_cascade_rule(rule: CascadeRule) -> Result<()> {
    let mut config = load_remotes_config()?;

    // Remove existing rule with same name
    config.cascade_rules.retain(|r| r.name != rule.name);
    config.cascade_rules.push(rule);

    save_remotes_config(&config)?;
    Ok(())
}

/// Remove a cascade rule from the configuration
pub fn remove_cascade_rule(name: &str) -> Result<bool> {
    let mut config = load_remotes_config()?;
    let initial_len = config.cascade_rules.len();
    config.cascade_rules.retain(|r| r.name != name);

    if config.cascade_rules.len() < initial_len {
        save_remotes_config(&config)?;
        Ok(true)
    } else {
        Ok(false)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_remotes_config() {
        let toml_content = r#"
[settings]
poll_interval = 30

[[remotes]]
name = "ci-server"
url = "https://ci.example.com:8787"
api_key = "notif_abc123"
mode = "passthrough"
tags = ["ci", "build"]

[[remotes]]
name = "work-server"
url = "https://work.example.com:8787"
api_key = "notif_def456"
mode = "store"
auto_approve = true

[[cascade]]
name = "forward-alerts"
match_tags = ["alert", "urgent"]
target_url = "https://downstream.example.com:8787"
target_api_key = "notif_xyz"
add_tags = ["forwarded"]
"#;

        let config: RemotesConfig = toml::from_str(toml_content).unwrap();

        assert_eq!(config.settings.poll_interval, 30);
        assert_eq!(config.remotes.len(), 2);
        assert_eq!(config.remotes[0].name, "ci-server");
        assert_eq!(config.remotes[0].mode, RemoteMode::Passthrough);
        assert_eq!(config.remotes[1].mode, RemoteMode::Store);
        assert!(config.remotes[1].auto_approve);
        assert_eq!(config.cascade_rules.len(), 1);
        assert_eq!(config.cascade_rules[0].name, "forward-alerts");
    }
}
