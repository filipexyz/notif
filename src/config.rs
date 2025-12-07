use std::env;
use std::fs;
use std::path::PathBuf;

use crate::models::TagFilter;

const CONFIG_FILE: &str = ".notif.json";

/// Load project config from .notif.json
/// Checks $CLAUDE_PROJECT_DIR first, then current directory
pub fn load_project_config() -> Option<TagFilter> {
    // Try $CLAUDE_PROJECT_DIR first (set by Claude Code hooks)
    if let Ok(project_dir) = env::var("CLAUDE_PROJECT_DIR") {
        let config_path = PathBuf::from(project_dir).join(CONFIG_FILE);
        if let Some(config) = load_config_from_path(&config_path) {
            return Some(config);
        }
    }

    // Fall back to current directory
    let cwd = env::current_dir().ok()?;
    let config_path = cwd.join(CONFIG_FILE);
    load_config_from_path(&config_path)
}

fn load_config_from_path(path: &PathBuf) -> Option<TagFilter> {
    let content = fs::read_to_string(path).ok()?;
    serde_json::from_str(&content).ok()
}

/// Save project config to .notif.json in the current directory
pub fn save_project_config(config: &TagFilter) -> anyhow::Result<PathBuf> {
    let cwd = env::current_dir()?;
    let config_path = cwd.join(CONFIG_FILE);
    let content = serde_json::to_string_pretty(config)?;
    fs::write(&config_path, content)?;
    Ok(config_path)
}
