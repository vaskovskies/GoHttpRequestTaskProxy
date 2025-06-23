CREATE TABLE IF NOT EXISTS tasks (
    id BIGSERIAL PRIMARY KEY, 
    status VARCHAR(50) NOT NULL,
    http_status_code INT NOT NULL,
    request_headers JSONB NOT NULL,
    response_headers JSONB,
    request_body TEXT NOT NULL,
    response_body TEXT NULL,
    length BIGINT NOT NULL,
    scheduled_start_time TIMESTAMP NOT NULL,
    scheduled_end_time TIMESTAMP NULL
);
