-- name: CreateConversationMember :one
INSERT INTO conversation_members (
    conversation_id,
    user_id,
    role,
    last_read_message_id
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: BatchCreateMembers :many
INSERT INTO conversation_members (
    conversation_id,
    user_id,
    role,
    last_read_message_id
) VALUES (
    -- 使用 sqlc 的 slice 参数
    $1,                   -- conversation_id
    unnest($2::bigint[]), -- user_ids
    $3,                   -- roles
    $4                    -- last_read_message_id
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

-- name: CountUnreadMessages :one
SELECT COUNT(*) 
FROM messages 
WHERE conversation_id = $1 AND id > $2 AND sender_id != $3
LIMIT 100;

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


-- name: CountConversationMembers :one
SELECT COUNT(*) 
FROM conversation_members 
WHERE conversation_id = $1;
