-- name: CreateConversation :one
INSERT INTO conversations (
    type,
    name,
    owner_id
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations
WHERE id = $1 LIMIT 1;

-- name: GetConversationForUpdate :one
SELECT * FROM conversations
WHERE id = $1 LIMIT 1
FOR NO KEY UPDATE;

-- name: ListMyConversations :many
SELECT c.* 
FROM conversations c
JOIN conversation_members cm ON c.id = cm.conversation_id
WHERE cm.user_id = $1
ORDER BY c.updated_at DESC; -- 按最后活跃时间排序
