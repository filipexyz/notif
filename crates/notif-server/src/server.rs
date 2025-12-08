use axum::{
    middleware,
    routing::{delete, get, post, put},
    Extension, Router,
};
use std::sync::Arc;
use tower_http::cors::{Any, CorsLayer};
use tower_http::trace::TraceLayer;

use crate::auth::{auth_middleware, ApiKeys};
use crate::config::Config;
use crate::routes::{api, webhook};

#[derive(Clone)]
pub struct AppState {
    pub config: Arc<Config>,
}

pub fn create_app(config: Config) -> Router {
    let api_keys = ApiKeys(Arc::new(
        config.get_api_keys().iter().map(|s| s.to_string()).collect(),
    ));

    let state = AppState {
        config: Arc::new(config),
    };

    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    Router::new()
        // Notification API
        .route("/notifications", get(api::list_notifications))
        .route("/notifications", post(webhook::create_notification))
        .route("/notifications/pull", get(api::pull_notifications))
        .route("/notifications/{id}", get(api::get_notification))
        .route("/notifications/{id}/approve", put(api::approve_notification))
        .route("/notifications/{id}/dismiss", put(api::dismiss_notification))
        .route("/notifications/{id}", delete(api::delete_notif))
        .route("/notifications/approve-all", post(api::approve_all))
        .route("/notifications/dismiss-all", post(api::dismiss_all))
        // Middleware
        .layer(middleware::from_fn(auth_middleware))
        .layer(Extension(api_keys))
        .layer(cors)
        .layer(TraceLayer::new_for_http())
        .with_state(state)
}

pub async fn run_server(config: Config) -> anyhow::Result<()> {
    use notif_core::init_db;
    use tracing_subscriber::{layer::SubscriberExt, util::SubscriberInitExt};

    // Initialize logging
    tracing_subscriber::registry()
        .with(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| "notif_server=info,tower_http=info".into()),
        )
        .with(tracing_subscriber::fmt::layer())
        .init();

    // Initialize database
    init_db()?;

    let addr = format!("{}:{}", config.server.host, config.server.port);
    let listener = tokio::net::TcpListener::bind(&addr).await?;

    tracing::info!("Server running on http://{}", addr);

    if config.get_api_keys().is_empty() {
        tracing::warn!("No API keys configured - server is running in open mode!");
        tracing::warn!("Set NOTIF_API_KEY or configure keys in ~/.notif/server.toml");
    }

    let app = create_app(config);
    axum::serve(listener, app).await?;

    Ok(())
}
