-- Tracks per-recipient delivery so unread history can be recovered after a
-- gateway disconnect or Redis Pub/Sub outage.
CREATE TABLE IF NOT EXISTS message_deliveries (
    message_id   uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users(id),
    delivered_at timestamptz,
    PRIMARY KEY (message_id, user_id)
);

CREATE INDEX IF NOT EXISTS message_deliveries_unread
    ON message_deliveries (user_id, message_id)
    WHERE delivered_at IS NULL;

-- Backfill existing message recipients when applying this migration to a
-- database that already has conversation history.
INSERT INTO message_deliveries (message_id, user_id, delivered_at)
SELECT m.id, cm.user_id, NOW()
FROM messages m
JOIN conversation_members cm ON cm.conversation_id=m.conversation_id AND cm.left_at IS NULL
WHERE cm.user_id <> m.sender_id
ON CONFLICT (message_id, user_id) DO NOTHING;
