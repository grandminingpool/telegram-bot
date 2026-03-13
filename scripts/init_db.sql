-- Run this script as a PostgreSQL superuser (e.g. postgres) before running migrations.
-- Usage: psql -U postgres -f scripts/init_db.sql

-- Create user
CREATE USER pool_telegram_bot WITH PASSWORD 'strongpass';

-- Create database owned by the new user
CREATE DATABASE pool_telegram_bot OWNER pool_telegram_bot;

-- Connect to the new database to configure permissions
\connect pool_telegram_bot

-- Revoke all default permissions from public
REVOKE ALL ON SCHEMA public FROM PUBLIC;

-- Grant schema usage and table permissions to the user
GRANT USAGE, CREATE ON SCHEMA public TO pool_telegram_bot;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO pool_telegram_bot;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO pool_telegram_bot;

-- Ensure future tables and sequences created in this schema are also accessible
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT ALL PRIVILEGES ON TABLES TO pool_telegram_bot;

ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT ALL PRIVILEGES ON SEQUENCES TO pool_telegram_bot;
