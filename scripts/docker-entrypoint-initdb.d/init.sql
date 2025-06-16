CREATE TABLE IF NOT EXISTS tasks (
    id BIGSERIAL PRIMARY KEY, 
    status VARCHAR(50) NOT NULL,
    http_status_code INT NOT NULL,
    headers JSONB NOT NULL,
    body TEXT NOT NULL,
    length BIGINT NOT NULL,
    scheduled_start_time TIMESTAMP NOT NULL,
    scheduled_end_time TIMESTAMP NOT NULL
);
