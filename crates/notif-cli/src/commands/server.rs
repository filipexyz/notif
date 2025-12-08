use anyhow::Result;
use notif_server::{generate_api_key, run_server, Config};

pub fn run(host: Option<String>, port: Option<u16>) -> Result<()> {
    let mut config = Config::load()?;

    // Apply CLI overrides
    if let Some(h) = host {
        config.server.host = h;
    }
    if let Some(p) = port {
        config.server.port = p;
    }

    // Run the async server
    tokio::runtime::Runtime::new()?.block_on(async { run_server(config).await })
}

pub fn keygen() -> Result<()> {
    let key = generate_api_key();
    println!("{}", key);
    Ok(())
}
