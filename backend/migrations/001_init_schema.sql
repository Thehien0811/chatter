CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username            text NOT NULL UNIQUE,
    display_name        text NOT NULL,
    email               text UNIQUE,
    password_hash       text NOT NULL,
    avatar_media_id     uuid,
    status_message      text,
    presence            text NOT NULL DEFAULT 'offline'
                        CHECK (presence IN ('online', 'away', 'offline', 'do_not_disturb')),
    last_seen_at        timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE TABLE friend_requests (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    sender_id           uuid NOT NULL REFERENCES users(id),
    recipient_id        uuid NOT NULL REFERENCES users(id),
    status              text NOT NULL DEFAULT 'pending'
                        CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    created_at          timestamptz NOT NULL DEFAULT now(),
    responded_at        timestamptz,
    CHECK (sender_id <> recipient_id)
);

CREATE UNIQUE INDEX friend_requests_one_pending_direction
    ON friend_requests (sender_id, recipient_id)
    WHERE status = 'pending';

CREATE INDEX friend_requests_inbox
    ON friend_requests (recipient_id, created_at DESC)
    WHERE status = 'pending';

CREATE TABLE friendships (
    user_a_id           uuid NOT NULL REFERENCES users(id),
    user_b_id           uuid NOT NULL REFERENCES users(id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    created_from_request_id uuid REFERENCES friend_requests(id),
    PRIMARY KEY (user_a_id, user_b_id),
    CHECK (user_a_id < user_b_id)
);

CREATE INDEX friendships_by_b
    ON friendships (user_b_id, user_a_id);

CREATE TABLE conversations (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    type                text NOT NULL CHECK (type IN ('direct', 'group')),
    title               text,
    created_by          uuid NOT NULL REFERENCES users(id),
    direct_user_low     uuid REFERENCES users(id),
    direct_user_high    uuid REFERENCES users(id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    updated_at          timestamptz NOT NULL DEFAULT now(),
    last_message_at     timestamptz,
    CHECK (
        (type = 'direct' AND direct_user_low IS NOT NULL AND direct_user_high IS NOT NULL
         AND direct_user_low < direct_user_high AND title IS NULL)
        OR (type = 'group' AND direct_user_low IS NULL AND direct_user_high IS NULL)
    )
);

CREATE UNIQUE INDEX conversations_unique_direct_pair
    ON conversations (direct_user_low, direct_user_high)
    WHERE type = 'direct';

CREATE TABLE conversation_members (
    conversation_id     uuid NOT NULL REFERENCES conversations(id),
    user_id             uuid NOT NULL REFERENCES users(id),
    role                text NOT NULL DEFAULT 'member'
                        CHECK (role IN ('owner', 'admin', 'member')),
    joined_at           timestamptz NOT NULL DEFAULT now(),
    left_at             timestamptz,
    last_read_message_id uuid,
    muted_until         timestamptz,
    PRIMARY KEY (conversation_id, user_id)
);

CREATE INDEX conversation_members_user_active
    ON conversation_members (user_id, conversation_id)
    WHERE left_at IS NULL;

CREATE TABLE messages (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id     uuid NOT NULL REFERENCES conversations(id),
    sender_id           uuid NOT NULL REFERENCES users(id),
    client_message_id   text,
    body                text,
    kind                text NOT NULL DEFAULT 'text'
                        CHECK (kind IN ('text', 'image', 'video', 'file', 'system')),
    reply_to_message_id uuid REFERENCES messages(id),
    created_at          timestamptz NOT NULL DEFAULT now(),
    edited_at           timestamptz,
    deleted_at          timestamptz,
    CHECK (body IS NOT NULL OR kind <> 'text')
);

CREATE UNIQUE INDEX messages_client_idempotency
    ON messages (conversation_id, sender_id, client_message_id)
    WHERE client_message_id IS NOT NULL;

CREATE INDEX messages_conversation_timeline
    ON messages (conversation_id, created_at DESC, id DESC);

CREATE TABLE media_objects (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id            uuid NOT NULL REFERENCES users(id),
    storage_key         text NOT NULL UNIQUE,
    media_type          text NOT NULL,
    byte_size           bigint NOT NULL CHECK (byte_size >= 0),
    duration_ms         bigint CHECK (duration_ms IS NULL OR duration_ms >= 0),
    created_at          timestamptz NOT NULL DEFAULT now(),
    deleted_at          timestamptz
);

CREATE TABLE message_attachments (
    message_id          uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
    media_object_id     uuid NOT NULL REFERENCES media_objects(id),
    position            smallint NOT NULL DEFAULT 0 CHECK (position >= 0),
    PRIMARY KEY (message_id, position),
    UNIQUE (message_id, media_object_id)
);

ALTER TABLE users
    ADD CONSTRAINT users_avatar_media_fk
    FOREIGN KEY (avatar_media_id) REFERENCES media_objects(id);

CREATE TABLE calls (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    conversation_id     uuid NOT NULL REFERENCES conversations(id),
    initiated_by        uuid NOT NULL REFERENCES users(id),
    mode                text NOT NULL CHECK (mode IN ('audio', 'video')),
    status              text NOT NULL DEFAULT 'ringing'
                        CHECK (status IN ('ringing', 'active', 'ended', 'missed', 'declined', 'failed')),
    started_at          timestamptz,
    ended_at            timestamptz,
    created_at          timestamptz NOT NULL DEFAULT now(),
    CHECK (ended_at IS NULL OR started_at IS NULL OR ended_at >= started_at)
);

CREATE INDEX calls_conversation_history
    ON calls (conversation_id, created_at DESC);

CREATE TABLE call_participants (
    call_id             uuid NOT NULL REFERENCES calls(id) ON DELETE CASCADE,
    user_id             uuid NOT NULL REFERENCES users(id),
    status              text NOT NULL DEFAULT 'invited'
                        CHECK (status IN ('invited', 'joined', 'declined', 'left', 'missed')),
    invited_at          timestamptz NOT NULL DEFAULT now(),
    joined_at           timestamptz,
    left_at             timestamptz,
    PRIMARY KEY (call_id, user_id)
);

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER users_set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER conversations_set_updated_at
BEFORE UPDATE ON conversations
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();
