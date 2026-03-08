-- name: CreateMessage :one
INSERT INTO messages (
  conversation_id,
  sender_id,
  msg_type,
  content
) VALUES (
  $1, $2, $3, $4
) RETURNING *;

-- name: GetMessage :one
SELECT * FROM messages
WHERE id = $1 LIMIT 1;

-- name: ListMessages :many
SELECT * FROM messages
WHERE conversation_id = $1 
  AND is_deleted = false
  AND id < $2  -- 关键：基于 ID 过滤
ORDER BY id DESC -- 必须配合索引 (conversation_id, id)
LIMIT $3;

-- name: ListHistoryMessages :many
SELECT 
    id,
    conversation_id,
    sender_id,
    msg_type,
    content,
    created_at,
    is_deleted
FROM messages
WHERE 
    conversation_id = $1 
    AND is_deleted = FALSE
    -- 关键逻辑：如果提供了 cursor_id，只查比它小的 ID
    -- sqlc.narg 允许参数为 NULL，实现可选过滤
    AND (sqlc.narg('cursor_id')::bigint IS NULL OR id < sqlc.narg('cursor_id')::bigint)
ORDER BY id DESC  -- 必须按 ID 倒序（或时间倒序），保证一致性
LIMIT $2;         -- 只需要 Limit，不需要 Offset

-- name: ListMessagesBySender :many
SELECT * FROM messages
WHERE conversation_id = $1 and sender_id = $2 and is_deleted = false
ORDER BY created_at DESC
LIMIT $3
OFFSET $4;

-- name: DeleteMessage :one
UPDATE messages
SET is_deleted = true
WHERE id = $1
RETURNING *;

-- name: RecallMessage :one
UPDATE messages
SET is_deleted = true,
    content = '', -- 清空内容
    msg_type = 1  -- 转为文本类型（可选）
WHERE id = $1 
  AND sender_id = $2 -- 确保只能撤回自己的
  AND created_at > NOW() - INTERVAL '2 minutes' -- 时间限制
RETURNING *;
