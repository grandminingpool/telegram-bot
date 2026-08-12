DROP TABLE IF EXISTS sent_immature_block_notifications;

DROP TABLE IF EXISTS sent_payout_notifications;

DROP TABLE IF EXISTS payouts_notifications;

CREATE TABLE payouts_notifications (
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
