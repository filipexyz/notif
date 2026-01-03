//! Example: Emit an event to notif.sh

use notifsh::Notif;
use serde_json::json;

#[tokio::main]
async fn main() -> notifsh::Result<()> {
    // Create client from NOTIF_API_KEY environment variable
    let client = Notif::from_env()?;

    // Emit an event
    let response = client
        .emit(
            "orders.created",
            json!({
                "order_id": "ord_123",
                "customer": "john@example.com",
                "total": 99.99
            }),
        )
        .await?;

    println!("Event published!");
    println!("  ID: {}", response.id);
    println!("  Topic: {}", response.topic);
    println!("  Created: {}", response.created_at);

    Ok(())
}
