CREATE TABLE IF NOT EXISTS blockchains (
    coin VARCHAR(32) NOT NULL PRIMARY KEY,
    name VARCHAR(128) NOT NULL,
    ticker VARCHAR(16) NOT NULL,
    atomic_unit SMALLINT NOT NULL,
    example_wallet VARCHAR(256) NOT NULL,
    pool_api_url VARCHAR(64) NOT NULL,
    pool_api_tls_ca VARCHAR(64) NOT NULL,
    pool_api_server_name VARCHAR(128) NOT NULL
);

ALTER TABLE blockchains ADD CONSTRAINT blockchains_unique_name UNIQUE (name);
ALTER TABLE blockchains ADD CONSTRAINT blockchains_unique_ticker UNIQUE (ticker);
ALTER TABLE blockchains ADD CONSTRAINT blockchains_unique_pool_api_url UNIQUE (pool_api_url);

CREATE TABLE IF NOT EXISTS users (
    id BIGINT NOT NULL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    lang VARCHAR(16) NOT NULL,
    payouts_notify BOOLEAN NOT NULL DEFAULT true,
    block_notify BOOLEAN NOT NULL DEFAULT true
);

ALTER TABLE users ADD CONSTRAINT users_unique_chat_id UNIQUE (chat_id);

CREATE TABLE IF NOT EXISTS user_actions (
    user_id BIGINT NOT NULL PRIMARY KEY,
    action VARCHAR(128) NOT NULL,
    payload TEXT
);

CREATE TABLE IF NOT EXISTS user_wallets (
    id BIGINT NOT NULL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    blockchain_coin VARCHAR(32) NOT NULL,
    wallet VARCHAR(256) NOT NULL,
    added_at TIMESTAMP NOT NULL DEFAULT NOW()
);

ALTER TABLE user_wallets ADD CONSTRAINT user_wallets_user_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE user_wallets ADD CONSTRAINT user_wallets_blockchain_fkey FOREIGN KEY (blockchain_coin) REFERENCES blockchains(coin) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE user_wallets ADD CONSTRAINT user_wallets_unique_wallet UNIQUE (user_id, blockchain_coin, wallet);

CREATE SEQUENCE user_wallets_id_seq
    AS BIGINT
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
    
ALTER SEQUENCE user_wallets_id_seq OWNED BY user_wallets.id;
ALTER TABLE ONLY user_wallets ALTER COLUMN id SET DEFAULT nextval('user_wallets_id_seq');
SELECT setval('user_wallets_id_seq', 1);

CREATE TABLE IF NOT EXISTS wallet_workers (
    wallet_id BIGINT NOT NULL,
    worker VARCHAR(64) NOT NULL,
    region VARCHAR(32) NOT NULL,
    solo BOOLEAN NOT NULL,
    connected_at TIMESTAMP NOT NULL,
    PRIMARY KEY(wallet_id, worker)
);

ALTER TABLE wallet_workers ADD CONSTRAINT wallet_workers_wallet_fkey FOREIGN KEY (wallet_id) REFERENCES user_wallets(id) ON UPDATE CASCADE ON DELETE CASCADE;
ALTER TABLE wallet_workers ADD CONSTRAINT wallet_workers_unique_worker UNIQUE (wallet_id, worker);

CREATE TABLE IF NOT EXISTS user_feedback (
    user_id BIGINT NOT NULL,
    first_name VARCHAR(255),
    last_name VARCHAR(255),
    username VARCHAR(32),
    report_message TEXT NOT NULL,
    added_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS payouts_notifications (
    id BIGINT NOT NULL PRIMARY KEY,
    executed_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE SEQUENCE payouts_notifications_id_seq
    AS BIGINT
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;
    
ALTER SEQUENCE payouts_notifications_id_seq OWNED BY payouts_notifications.id;
ALTER TABLE ONLY payouts_notifications ALTER COLUMN id SET DEFAULT nextval('payouts_notifications_id_seq');
SELECT setval('payouts_notifications_id_seq', 1);

CREATE INDEX payouts_notifications_executed_time_idx ON payouts_notifications USING BTREE(executed_at);