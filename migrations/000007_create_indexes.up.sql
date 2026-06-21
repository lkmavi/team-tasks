-- tasks: filter by team + status (primary task list query)
CREATE INDEX idx_tasks_team_status  ON tasks (team_id, status);

-- tasks: filter by assignee
CREATE INDEX idx_tasks_assignee     ON tasks (assignee_id);

-- tasks: sort/filter by creation date (analytics, pagination)
CREATE INDEX idx_tasks_created_at   ON tasks (created_at);

-- task_history: fetch history entries for a task
CREATE INDEX idx_task_history_task  ON task_history (task_id);

-- task_comments: fetch comments for a task
CREATE INDEX idx_task_comments_task ON task_comments (task_id);

-- team_members: find all teams a user belongs to
CREATE INDEX idx_team_members_user  ON team_members (user_id);
