//! Example: Subscribe to events from notif.sh

use futures::StreamExt;
use notifsh::{Notif, SubscribeOptions};

#[tokio::main]
async fn main() -> notifsh::Result<()> {
    // Create client from NOTIF_API_KEY environment variable
    let client = Notif::from_env()?;

    println!("Subscribing to orders.*...");

    // Simple subscription (auto-ack enabled)
    // let mut stream = client.subscribe(&["orders.*"]).await?;

    // Or with options:
    let mut stream = client
        .subscribe_with_options(
            &["orders.*"],
            SubscribeOptions::new()
                .auto_ack(false) // Manual acknowledgment
                .from("latest"), // Start from new events only
        )
        .await?;

    println!("Waiting for events... (Ctrl+C to exit)");

    while let Some(result) = stream.next().await {
        match result {
            Ok(event) => {
                println!("\nReceived event:");
                println!("  ID: {}", event.id);
                println!("  Topic: {}", event.topic);
                println!("  Data: {}", event.data);
                println!("  Attempt: {}/{}", event.attempt, event.max_attempts);

                // Acknowledge the event
                event.ack().await?;
                println!("  ✓ Acknowledged");
            }
            Err(e) => {
                eprintln!("Error: {}", e);
            }
        }
    }

    Ok(())
}
