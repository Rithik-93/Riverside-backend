-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) NOT NULL,
    username VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    password VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);

-- Create sessions table
CREATE TABLE IF NOT EXISTS sessions (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL,
    access_token VARCHAR(255) NOT NULL,
    refresh_token VARCHAR(255) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_access_token ON sessions(access_token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_refresh_token ON sessions(refresh_token);

-- Create pods table
CREATE TABLE IF NOT EXISTS pods (
    id BIGSERIAL PRIMARY KEY,
    host_user_id VARCHAR(255) NOT NULL,
    target_user_id VARCHAR(255),
    participant_count INTEGER DEFAULT 1,
    status VARCHAR(50) DEFAULT 'ONGOING',
    is_recording BOOLEAN DEFAULT FALSE,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMP,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_pods_host_user_id ON pods(host_user_id);
CREATE INDEX IF NOT EXISTS idx_pods_target_user_id ON pods(target_user_id);
CREATE INDEX IF NOT EXISTS idx_pods_status ON pods(status);

-- Create pod_participants table
CREATE TABLE IF NOT EXISTS pod_participants (
    pod_id BIGINT NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    joined_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (pod_id, user_id)
);


