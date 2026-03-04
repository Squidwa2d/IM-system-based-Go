-- name: CreateConversationMember :one
INSERT INTO conversation_members (
    conversation_id,
    user_id,
    role
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetConversationMember :one
SELECT * FROM conversation_members
WHERE conversation_id = $1 AND user_id = $2 LIMIT 1;

-- name: ListConversationMembers :many
SELECT * FROM conversation_members
where conversation_id = $1;

-- name: UpdateReadStatus :exec
UPDATE conversation_members
SET last_read_message_id = $3, 
    last_active_at = NOW()
WHERE conversation_id = $1 
  AND user_id = $2;

-- name: RemoveConversationMember :exec
DELETE FROM conversation_members
WHERE conversation_id = $1 
  AND user_id = $2;

-- name: UpdateMemberRole :exec
UPDATE conversation_members
SET role = $3
WHERE conversation_id = $1 
  AND user_id = $2;

-- name: CheckMemberExists :one
SELECT EXISTS (
    SELECT 1 FROM conversation_members
    WHERE conversation_id = $1 AND user_id = $2
) as is_member;

-- name: BatchCreateMembers :many
INSERT INTO conversation_members (
    conversation_id,
    user_id,
    role
) VALUES (
    -- 使用 sqlc 的 slice 参数
    $1,                   -- conversation_id
    unnest($2::bigint[]), -- user_ids
    $3                    -- roles
) RETURNING *;

-- name: CountConversationMembers :one
SELECT COUNT(*) 
FROM conversation_members 
WHERE conversation_id = $1;

-- name: CountUnreadMessages :one
SELECT COUNT(*) 
FROM messages 
WHERE conversation_id = $1 AND id > $2;