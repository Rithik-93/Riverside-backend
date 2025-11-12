-- Rollback: Recreate unused tables
CREATE TABLE IF NOT EXISTS call_sessions (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    room_id VARCHAR(255) NOT NULL,
    target_user_id VARCHAR(255),
    started_at TIMESTAMP NOT NULL,
    ended_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    duration_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_call_sessions_client_id ON call_sessions(client_id);
CREATE INDEX IF NOT EXISTS idx_call_sessions_room_id ON call_sessions(room_id);
CREATE INDEX IF NOT EXISTS idx_call_sessions_target_user_id ON call_sessions(target_user_id);
CREATE INDEX IF NOT EXISTS idx_call_sessions_user_id ON call_sessions(user_id);

CREATE TABLE IF NOT EXISTS client_connections (
    id BIGSERIAL PRIMARY KEY,
    client_id VARCHAR(255) NOT NULL UNIQUE,
    user_id VARCHAR(255) NOT NULL,
    connected_at TIMESTAMP NOT NULL,
    disconnected_at TIMESTAMP,
    user_agent VARCHAR(500),
    ip_address VARCHAR(45),
    is_active BOOLEAN DEFAULT TRUE,
    duration_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_client_connections_user_id ON client_connections(user_id);

CREATE TABLE IF NOT EXISTS room_participations (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    client_id VARCHAR(255) NOT NULL,
    room_id VARCHAR(255) NOT NULL,
    joined_at TIMESTAMP NOT NULL,
    left_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_room_participations_client_id ON room_participations(client_id);
CREATE INDEX IF NOT EXISTS idx_room_participations_room_id ON room_participations(room_id);
CREATE INDEX IF NOT EXISTS idx_room_participations_user_id ON room_participations(user_id);

CREATE TABLE IF NOT EXISTS signaling_events (
    id BIGSERIAL PRIMARY KEY,
    event_type VARCHAR(255) NOT NULL,
    user_id VARCHAR(255),
    client_id VARCHAR(255) NOT NULL,
    room_id VARCHAR(255),
    data JSONB,
    timestamp BIGINT NOT NULL,
    processed_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_signaling_events_client_id ON signaling_events(client_id);
CREATE INDEX IF NOT EXISTS idx_signaling_events_event_type ON signaling_events(event_type);
CREATE INDEX IF NOT EXISTS idx_signaling_events_room_id ON signaling_events(room_id);
CREATE INDEX IF NOT EXISTS idx_signaling_events_user_id ON signaling_events(user_id);

