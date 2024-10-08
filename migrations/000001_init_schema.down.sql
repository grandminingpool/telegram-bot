DROP TABLE IF EXISTS blockchains;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS user_actions;
DROP TABLE IF EXISTS user_wallets;
DROP SEQUENCE IF EXISTS user_wallets_id_seq;
DROP TABLE IF EXISTS wallet_workers;
DROP TABLE IF EXISTS user_feedback;
DROP TABLE IF EXISTS payments_notifications;
DROP SEQUENCE IF EXISTS payments_notifications_id_seq;
DROP INDEX IF EXISTS payments_notifications_executed_time_idx;