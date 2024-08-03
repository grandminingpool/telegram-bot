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

CREATE TABLE IF NOT EXISTS user_settings (
    id BIGINT NOT NULL PRIMARY KEY,
    lang VARCHAR(16) NOT NULL,
    payouts_notify BOOLEAN NOT NULL DEFAULT true,
    block_notify BOOLEAN NOT NULL DEFAULT true,
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS user_actions (
    id BIGINT NOT NULL PRIMARY KEY,
    action VARCHAR(128) NOT NULL,
    payload TEXT
);

CREATE TABLE IF NOT EXISTS user_wallets (
    user_id BIGINT NOT NULL PRIMARY KEY,
    blockchain_coin VARCHAR(32)NOT NULL,
    wallet VARCHAR(256) NOT NULL
);

ALTER TABLE user_wallets ADD CONSTRAINT user_wallets_unique_wallet UNIQUE (wallet);
ALTER TABLE user_wallets ADD CONSTRAINT user_wallets_blockchain_fkey FOREIGN KEY (blockchain_coin) REFERENCES blockchains(coin) ON UPDATE CASCADE ON DELETE CASCADE;