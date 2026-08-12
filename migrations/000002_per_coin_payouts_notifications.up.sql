DROP TABLE IF EXISTS payouts_notifications;
DROP SEQUENCE IF EXISTS payouts_notifications_id_seq;

CREATE TABLE payouts_notifications (
    coin VARCHAR(32) NOT NULL PRIMARY KEY REFERENCES blockchains(coin) ON UPDATE CASCADE ON DELETE CASCADE,
    last_payouts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_solo_payouts_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sent_payout_notifications (
    coin VARCHAR(32) NOT NULL REFERENCES blockchains(coin) ON UPDATE CASCADE ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    tx_hash VARCHAR(128) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (coin, user_id, tx_hash)
);

CREATE INDEX sent_payout_notifications_sent_at_idx ON sent_payout_notifications USING BTREE(sent_at);

CREATE TABLE sent_immature_block_notifications (
    coin VARCHAR(32) NOT NULL REFERENCES blockchains(coin) ON UPDATE CASCADE ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
    block_hash VARCHAR(128) NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (coin, user_id, block_hash)
);

CREATE INDEX sent_immature_block_notifications_sent_at_idx ON sent_immature_block_notifications USING BTREE(sent_at);
