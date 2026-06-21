CREATE TABLE IF NOT EXISTS task_comments (
    id         BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    task_id    BINARY(16)       NOT NULL,
    user_id    BINARY(16)       NOT NULL,
    body       TEXT             NOT NULL,
    created_at DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    CONSTRAINT fk_task_comments_task FOREIGN KEY (task_id) REFERENCES tasks (id),
    CONSTRAINT fk_task_comments_user FOREIGN KEY (user_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
