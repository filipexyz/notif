use anyhow::{Context, Result};
use colored::Colorize;
use notif_core::{save_project_config, FilterMode, TagFilter};
use serde_json::{json, Value};
use std::fs;
use std::path::PathBuf;

fn get_claude_settings_path() -> PathBuf {
    let home = dirs_next::home_dir().unwrap_or_else(|| PathBuf::from("."));
    home.join(".claude").join("settings.json")
}

pub fn run(tags: &[String]) -> Result<()> {
    // If tags provided, create .notif.json in current directory
    if !tags.is_empty() {
        let filter = TagFilter {
            tags: tags.to_vec(),
            mode: FilterMode::Include,
        };
        let config_path = save_project_config(&filter)?;
        println!("{}", "Created project config!".green().bold());
        println!("Path: {}", config_path.display().to_string().cyan());
        println!("Tags: {}", tags.join(", ").yellow());
        println!();
    }

    let settings_path = get_claude_settings_path();

    // Ensure .claude directory exists
    if let Some(parent) = settings_path.parent() {
        fs::create_dir_all(parent).context("Could not create .claude directory")?;
    }

    // Read existing settings or create empty object
    let mut settings: Value = if settings_path.exists() {
        let content = fs::read_to_string(&settings_path)
            .context("Could not read existing settings.json")?;
        serde_json::from_str(&content).unwrap_or_else(|_| json!({}))
    } else {
        json!({})
    };

    // Ensure settings is an object
    if !settings.is_object() {
        settings = json!({});
    }
    let settings_obj = settings.as_object_mut().unwrap();

    // Check if hook already exists
    if let Some(hooks) = settings_obj.get("hooks") {
        if let Some(user_prompt_submit) = hooks.get("UserPromptSubmit") {
            if let Some(arr) = user_prompt_submit.as_array() {
                for item in arr {
                    if let Some(inner_hooks) = item.get("hooks") {
                        if let Some(inner_arr) = inner_hooks.as_array() {
                            for hook in inner_arr {
                                if let Some(cmd) = hook.get("command") {
                                    if cmd.as_str() == Some("notif hook") {
                                        println!(
                                            "{}",
                                            "Hook already configured in settings.json".yellow()
                                        );
                                        println!("Path: {}", settings_path.display());
                                        return Ok(());
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }

    // Build the hook configuration
    let hook_config = json!({
        "hooks": [{
            "type": "command",
            "command": "notif hook"
        }]
    });

    // Add or update hooks
    let hooks = settings_obj
        .entry("hooks")
        .or_insert_with(|| json!({}));

    let hooks_obj = hooks.as_object_mut().unwrap();

    let user_prompt_submit = hooks_obj
        .entry("UserPromptSubmit")
        .or_insert_with(|| json!([]));

    if let Some(arr) = user_prompt_submit.as_array_mut() {
        arr.push(hook_config);
    }

    // Write back
    let new_content = serde_json::to_string_pretty(&settings)?;
    fs::write(&settings_path, new_content).context("Could not write settings.json")?;

    println!("{}", "Hook configured successfully!".green().bold());
    println!();
    println!("Path: {}", settings_path.display().to_string().cyan());
    println!();
    println!("The {} hook will now inject approved notifications", "UserPromptSubmit".yellow());
    println!("into your Claude Code sessions.");
    println!();
    println!("Try it out:");
    println!("  {} notif add --approve \"Test notification\"", "$".dimmed());
    println!("  {} # then chat with Claude", "$".dimmed());

    Ok(())
}
