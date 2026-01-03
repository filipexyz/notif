//! WebSocket subscription implementation.

use std::pin::Pin;
use std::sync::Arc;
use std::task::{Context, Poll};

use futures_util::{SinkExt, Stream, StreamExt};
use tokio::sync::mpsc;
use tokio_tungstenite::{connect_async, tungstenite::Message};

use crate::client::NotifInner;
use crate::error::{NotifError, Result};
use crate::types::{
    AckMessage, AckWireMessage, Event, NackWireMessage, ServerMessage, SubscribeMessage,
    SubscribeOptions, SubscribeOptionsWire,
};

/// A stream of events from a subscription.
///
/// Implements `futures::Stream<Item = Result<Event>>`.
pub struct EventStream {
    event_rx: mpsc::Receiver<Result<Event>>,
    #[allow(dead_code)]
    ack_tx: mpsc::Sender<AckMessage>,
}

impl EventStream {
    /// Connect to the WebSocket and start receiving events.
    pub(crate) async fn connect(
        inner: Arc<NotifInner>,
        topics: &[&str],
        options: SubscribeOptions,
    ) -> Result<Self> {
        // Convert HTTP URL to WebSocket URL
        let ws_url = inner
            .server
            .replace("https://", "wss://")
            .replace("http://", "ws://");
        let ws_url = format!("{}/ws?token={}", ws_url, inner.api_key);

        // Connect to WebSocket
        let (ws_stream, _) = connect_async(&ws_url)
            .await
            .map_err(|e| NotifError::websocket(format!("connection failed: {}", e)))?;

        let (mut write, mut read) = ws_stream.split();

        // Send subscribe message
        let subscribe_msg = SubscribeMessage {
            action: "subscribe".to_string(),
            topics: topics.iter().map(|s| s.to_string()).collect(),
            options: Some(SubscribeOptionsWire {
                auto_ack: options.auto_ack,
                from: options.from.clone(),
                group: options.group.clone(),
            }),
        };

        let msg_json = serde_json::to_string(&subscribe_msg)?;
        write
            .send(Message::Text(msg_json))
            .await
            .map_err(|e| NotifError::websocket(format!("failed to send subscribe: {}", e)))?;

        // Wait for subscribed confirmation
        match read.next().await {
            Some(Ok(Message::Text(text))) => {
                let msg: ServerMessage = serde_json::from_str(&text)?;
                match msg.msg_type.as_str() {
                    "subscribed" => {
                        // Successfully subscribed
                    }
                    "error" => {
                        return Err(NotifError::api(
                            400,
                            msg.message.unwrap_or_else(|| "subscription error".to_string()),
                        ));
                    }
                    _ => {
                        return Err(NotifError::websocket(format!(
                            "unexpected message type: {}",
                            msg.msg_type
                        )));
                    }
                }
            }
            Some(Ok(_)) => {
                return Err(NotifError::websocket("unexpected message format"));
            }
            Some(Err(e)) => {
                return Err(NotifError::websocket(format!("WebSocket error: {}", e)));
            }
            None => {
                return Err(NotifError::websocket("connection closed unexpectedly"));
            }
        }

        // Create channels for events and acks
        let (event_tx, event_rx) = mpsc::channel::<Result<Event>>(100);
        let (ack_tx, mut ack_rx) = mpsc::channel::<AckMessage>(100);

        let ack_tx_for_events = if options.auto_ack { None } else { Some(ack_tx.clone()) };

        // Spawn background task to handle WebSocket messages
        tokio::spawn(async move {
            loop {
                tokio::select! {
                    // Handle incoming messages
                    msg = read.next() => {
                        match msg {
                            Some(Ok(Message::Text(text))) => {
                                match serde_json::from_str::<ServerMessage>(&text) {
                                    Ok(server_msg) => {
                                        if server_msg.msg_type == "event" {
                                            // Validate required fields
                                            let (id, topic) = match (server_msg.id, server_msg.topic) {
                                                (Some(id), Some(topic)) => (id, topic),
                                                _ => {
                                                    let _ = event_tx.send(Err(NotifError::websocket(
                                                        "malformed event: missing id or topic"
                                                    ))).await;
                                                    continue;
                                                }
                                            };
                                            let event = Event {
                                                id,
                                                topic,
                                                data: server_msg.data.unwrap_or(serde_json::Value::Null),
                                                timestamp: server_msg.timestamp.unwrap_or_else(chrono::Utc::now),
                                                attempt: server_msg.attempt.unwrap_or(1),
                                                max_attempts: server_msg.max_attempts.unwrap_or(3),
                                                ack_tx: ack_tx_for_events.clone(),
                                            };
                                            if event_tx.send(Ok(event)).await.is_err() {
                                                break;
                                            }
                                        } else if server_msg.msg_type == "error" {
                                            let err = NotifError::api(
                                                400,
                                                server_msg.message.unwrap_or_else(|| "unknown error".to_string()),
                                            );
                                            let _ = event_tx.send(Err(err)).await;
                                        }
                                    }
                                    Err(e) => {
                                        let _ = event_tx.send(Err(NotifError::Serialization(e))).await;
                                    }
                                }
                            }
                            Some(Ok(Message::Close(_))) => {
                                break;
                            }
                            Some(Err(e)) => {
                                let _ = event_tx.send(Err(NotifError::websocket(e.to_string()))).await;
                                break;
                            }
                            None => break,
                            _ => {}
                        }
                    }
                    // Handle outgoing ack/nack messages
                    ack_msg = ack_rx.recv() => {
                        match ack_msg {
                            Some(AckMessage::Ack { id }) => {
                                let msg = AckWireMessage {
                                    action: "ack".to_string(),
                                    id,
                                };
                                if let Ok(json) = serde_json::to_string(&msg) {
                                    let _ = write.send(Message::Text(json)).await;
                                }
                            }
                            Some(AckMessage::Nack { id, retry_in }) => {
                                let msg = NackWireMessage {
                                    action: "nack".to_string(),
                                    id,
                                    retry_in,
                                };
                                if let Ok(json) = serde_json::to_string(&msg) {
                                    let _ = write.send(Message::Text(json)).await;
                                }
                            }
                            None => break,
                        }
                    }
                }
            }
        });

        Ok(Self { event_rx, ack_tx })
    }
}

impl Stream for EventStream {
    type Item = Result<Event>;

    fn poll_next(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Option<Self::Item>> {
        Pin::new(&mut self.event_rx).poll_recv(cx)
    }
}
