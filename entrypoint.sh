#!/bin/sh
set -e
echo "Running database migrations..."
goose -dir /app/migrations postgres "$DATABASE_URL" up
echo "Migrations complete. Starting server..."
exec /app/notifd
