CREATE TABLE IF NOT EXISTS teams (
    id         BINARY(16)   NOT NULL,
    name       VARCHAR(255) NOT NULL,
    created_by BINARY(16)   NOT NULL,
    created_at DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    CONSTRAINT fk_teams_created_by FOREIGN KEY (created_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
