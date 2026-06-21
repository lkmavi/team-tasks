CREATE TABLE IF NOT EXISTS tasks (
    id          BINARY(16)                             NOT NULL,
    team_id     BINARY(16)                             NOT NULL,
    created_by  BINARY(16)                             NOT NULL,
    assignee_id BINARY(16)                             NULL,
    title       VARCHAR(500)                           NOT NULL,
    description TEXT                                   NULL,
    status      ENUM('todo','in_progress','done')      NOT NULL DEFAULT 'todo',
    priority    ENUM('low','medium','high')             NOT NULL DEFAULT 'medium',
    due_date    DATE                                   NULL,
    created_at  DATETIME(3)                            NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at  DATETIME(3)                            NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    CONSTRAINT fk_tasks_team     FOREIGN KEY (team_id)     REFERENCES teams (id),
    CONSTRAINT fk_tasks_creator  FOREIGN KEY (created_by)  REFERENCES users (id),
    CONSTRAINT fk_tasks_assignee FOREIGN KEY (assignee_id) REFERENCES users (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
