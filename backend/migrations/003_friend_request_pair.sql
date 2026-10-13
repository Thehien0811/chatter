-- A pending request is unique for an unordered pair, preventing simultaneous
-- reciprocal requests from creating an ambiguous state machine.
CREATE UNIQUE INDEX IF NOT EXISTS friend_requests_one_pending_pair
    ON friend_requests (LEAST(sender_id, recipient_id), GREATEST(sender_id, recipient_id))
    WHERE status = 'pending';
