-- Create recordings table
CREATE TABLE IF NOT EXISTS recordings (
    id BIGSERIAL PRIMARY KEY,
    pod_id BIGINT NOT NULL,
    recording_id VARCHAR(255) NOT NULL,
    s3_url VARCHAR(1000),
    duration_ms BIGINT DEFAULT 0,
    state VARCHAR(50) DEFAULT 'started',
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_recordings_recording_id_pod_id ON recordings(recording_id, pod_id);
CREATE INDEX IF NOT EXISTS idx_recordings_pod_id ON recordings(pod_id);
CREATE INDEX IF NOT EXISTS idx_recordings_state ON recordings(state);

-- Create user_recording_links table
CREATE TABLE IF NOT EXISTS user_recording_links (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    recording_id VARCHAR(255) NOT NULL,
    s3_url VARCHAR(1000) NOT NULL,
    file_size BIGINT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_user_recording_links_user_id ON user_recording_links(user_id);
CREATE INDEX IF NOT EXISTS idx_user_recording_links_recording_id ON user_recording_links(recording_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_recording_links_unique ON user_recording_links(user_id, recording_id);

