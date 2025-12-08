use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default)]
    pub server: ServerConfig,
    #[serde(default)]
    pub auth: AuthConfig,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ServerConfig {
    #[serde(default = "default_host")]
    pub host: String,
    #[serde(default = "default_port")]
    pub port: u16,
    #[serde(default)]
    pub db_path: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AuthConfig {
    #[serde(default)]
    pub api_key: Option<String>,
    #[serde(default)]
    pub keys: Vec<String>,
}

fn default_host() -> String {
    "127.0.0.1".to_string()
}

fn default_port() -> u16 {
    8787
}

impl Default for ServerConfig {
    fn default() -> Self {
        Self {
            host: default_host(),
            port: default_port(),
            db_path: None,
        }
    }
}

impl Default for AuthConfig {
    fn default() -> Self {
        Self {
            api_key: None,
            keys: Vec::new(),
        }
    }
}

impl Default for Config {
    fn default() -> Self {
        Self {
            server: ServerConfig::default(),
            auth: AuthConfig::default(),
        }
    }
}

impl Config {
    pub fn load() -> Result<Self> {
        // Try loading from file first
        if let Some(config_path) = Self::config_path() {
            if config_path.exists() {
                let content = fs::read_to_string(&config_path)
                    .with_context(|| format!("Failed to read config from {:?}", config_path))?;
                let mut config: Config = toml::from_str(&content)
                    .with_context(|| "Failed to parse config file")?;

                // Override with env vars
                config.apply_env_overrides();
                return Ok(config);
            }
        }

        // Fall back to env vars only
        let mut config = Config::default();
        config.apply_env_overrides();
        Ok(config)
    }

    fn apply_env_overrides(&mut self) {
        if let Ok(host) = std::env::var("NOTIF_SERVER_HOST") {
            self.server.host = host;
        }
        if let Ok(port) = std::env::var("NOTIF_SERVER_PORT") {
            if let Ok(p) = port.parse() {
                self.server.port = p;
            }
        }
        if let Ok(key) = std::env::var("NOTIF_API_KEY") {
            self.auth.api_key = Some(key);
        }
        if let Ok(db) = std::env::var("NOTIF_SERVER_DB") {
            self.server.db_path = Some(db);
        }
    }

    fn config_path() -> Option<PathBuf> {
        dirs_next::home_dir().map(|h| h.join(".notif").join("server.toml"))
    }

    pub fn get_api_keys(&self) -> Vec<&str> {
        let mut keys: Vec<&str> = self.auth.keys.iter().map(|s| s.as_str()).collect();
        if let Some(ref key) = self.auth.api_key {
            keys.push(key.as_str());
        }
        keys
    }

    pub fn db_path(&self) -> PathBuf {
        if let Some(ref path) = self.server.db_path {
            // Expand ~ to home directory
            let expanded = if path.starts_with("~/") {
                if let Some(home) = dirs_next::home_dir() {
                    home.join(&path[2..])
                } else {
                    PathBuf::from(path)
                }
            } else {
                PathBuf::from(path)
            };
            expanded
        } else {
            dirs_next::data_dir()
                .unwrap_or_else(|| PathBuf::from("."))
                .join("notif")
                .join("server.db")
        }
    }
}

pub fn generate_api_key() -> String {
    use rand::Rng;
    let mut rng = rand::thread_rng();
    let bytes: Vec<u8> = (0..24).map(|_| rng.gen()).collect();
    format!("notif_{}", hex::encode(&bytes))
}

// Add hex encoding manually since we don't want another dep
mod hex {
    pub fn encode(bytes: &[u8]) -> String {
        bytes.iter().map(|b| format!("{:02x}", b)).collect()
    }
}
