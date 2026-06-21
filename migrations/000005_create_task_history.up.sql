CREATE TABLE IF NOT EXISTS task_history (
    id         BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    task_id    BINARY(16)       NOT NULL,
    changed_by BINARY(16)       NOT NULL,
    field      VARCHAR(100)     NOT NULL,
    old_value  TEXT             NULL,
    new_value  TEXT             NULL,
    changed_at DATETIME(3)      NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    CONSTRAINT fk_task_history_task FOREIGN KEY (task_id)    REFERENCES tasks (id),
    CONSTRAINT fk_task_history_user FOREIGN KEY (changed_by) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
