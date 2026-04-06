-- name: SearchUsers :many
SELECT id, username, avatar_url, status, created_at, updated_at 
FROM users
WHERE username ILIKE $1 
   OR EXISTS (
       SELECT 1 FROM user_profiles up 
       WHERE up.user_id = users.id 
       AND (up.nickname ILIKE $1 OR up.signature ILIKE $1)
   )
ORDER BY username
LIMIT $2 OFFSET $3;

-- name: SearchUsersCount :one
SELECT COUNT(*) FROM users
WHERE username ILIKE $1 
   OR EXISTS (
       SELECT 1 FROM user_profiles up 
       WHERE up.user_id = users.id 
       AND (up.nickname ILIKE $1 OR up.signature ILIKE $1)
   );

-- name: GetUserDetail :one
SELECT u.id, u.username, u.avatar_url, u.status, u.created_at, u.updated_at,
       up.nickname, up.signature, up.gender, up.birthday
FROM users u
LEFT JOIN user_profiles up ON u.id = up.user_id
WHERE u.id = $1;

-- name: UpdateUserProfile :one
INSERT INTO user_profiles (
    user_id,
    nickname,
    signature,
    gender,
    birthday,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, NOW()
) ON CONFLICT (user_id) DO UPDATE SET
    nickname = EXCLUDED.nickname,
    signature = EXCLUDED.signature,
    gender = EXCLUDED.gender,
    birthday = EXCLUDED.birthday,
    updated_at = NOW()
RETURNING *;

-- name: UpdateUserInfo :one
UPDATE users
SET avatar_url = COALESCE($2, avatar_url),
    status = COALESCE($3, status),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: CreateFriendship :one
INSERT INTO friendships (
    user_id,
    friend_id,
    status
) VALUES (
    $1, $2, 'pending'
) ON CONFLICT (user_id, friend_id) DO UPDATE SET
    status = 'pending',
    updated_at = NOW()
RETURNING *;

-- name: GetFriendship :one
SELECT * FROM friendships
WHERE user_id = $1 AND friend_id = $2
LIMIT 1;

-- name: GetFriendshipBidirectional :one
SELECT * FROM friendships
WHERE (user_id = $1 AND friend_id = $2)
   OR (user_id = $2 AND friend_id = $1)
LIMIT 1;

-- name: AcceptFriendship :exec
UPDATE friendships
SET status = 'accepted',
    updated_at = NOW()
WHERE user_id = $1 AND friend_id = $2;

-- name: RejectFriendship :exec
UPDATE friendships
SET status = 'rejected',
    updated_at = NOW()
WHERE user_id = $1 AND friend_id = $2;

-- name: DeleteFriendship :exec
DELETE FROM friendships
WHERE (user_id = $1 AND friend_id = $2)
   OR (user_id = $2 AND friend_id = $1);

-- name: GetFriendList :many
SELECT u.id, u.username, u.avatar_url, u.status, u.created_at, u.updated_at,
       f.remark, f.created_at as friend_since
FROM friendships f
JOIN users u ON f.friend_id = u.id
WHERE f.user_id = $1 AND f.status = 'accepted' AND f.blocked = false
ORDER BY u.username
LIMIT $2 OFFSET $3;

-- name: GetFriendRequestList :many
SELECT u.id, u.username, u.avatar_url, f.created_at
FROM friendships f
JOIN users u ON f.user_id = u.id
WHERE f.friend_id = $1 AND f.status = 'pending'
ORDER BY f.created_at DESC;

-- name: UpdateFriendRemark :exec
UPDATE friendships
SET remark = $3,
    updated_at = NOW()
WHERE user_id = $1 AND friend_id = $2;

-- name: BlockFriend :exec
UPDATE friendships
SET blocked = true,
    updated_at = NOW()
WHERE (user_id = $1 AND friend_id = $2)
   OR (user_id = $2 AND friend_id = $1);

-- name: UnblockFriend :exec
UPDATE friendships
SET blocked = false,
    updated_at = NOW()
WHERE (user_id = $1 AND friend_id = $2)
   OR (user_id = $2 AND friend_id = $1);

-- name: CheckFriendshipExists :one
SELECT EXISTS (
    SELECT 1 FROM friendships
    WHERE ((user_id = $1 AND friend_id = $2)
       OR (user_id = $2 AND friend_id = $1))
    AND status = 'accepted'
    AND blocked = false
) as is_friend;

-- name: InviteGroupMember :one
INSERT INTO conversation_members (
    conversation_id,
    user_id,
    role,
    last_read_message_id
) VALUES (
    $1, $2, $3, $4
) ON CONFLICT (conversation_id, user_id) DO NOTHING
RETURNING *;

-- name: KickGroupMember :exec
DELETE FROM conversation_members
WHERE conversation_id = $1 AND user_id = $2;

-- name: LeaveGroup :exec
DELETE FROM conversation_members
WHERE conversation_id = $1 AND user_id = $2;

-- name: UpdateGroupInfo :one
UPDATE conversations
SET name = COALESCE($2, name),
    avatar_url = COALESCE($3, avatar_url),
    updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: TransferGroupOwner :exec
UPDATE conversation_members
SET role = CASE 
    WHEN user_id = $2 THEN 3
    WHEN user_id = $3 THEN 1
    ELSE role
END,
updated_at = NOW()
WHERE conversation_id = $1 
AND user_id IN ($2, $3);

-- name: CreateGroupAnnouncement :one
INSERT INTO group_announcements (
    conversation_id,
    publisher_id,
    content
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetGroupAnnouncement :one
SELECT * FROM group_announcements
WHERE conversation_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: UpdateGroupAnnouncement :one
UPDATE group_announcements
SET content = $3,
    updated_at = NOW()
WHERE conversation_id = $1 AND publisher_id = $2
RETURNING *;

-- name: CreateMessageForward :one
INSERT INTO message_forwards (
    original_message_id,
    forwarded_by,
    target_conversation_id
) VALUES (
    $1, $2, $3
) RETURNING *;

-- name: GetMessageForwards :many
SELECT mf.*, m.content, m.msg_type
FROM message_forwards mf
JOIN messages m ON mf.original_message_id = m.id
WHERE mf.forwarded_by = $1
ORDER BY mf.forwarded_at DESC
LIMIT $2 OFFSET $3;

-- name: GetPrivateConversation :one
SELECT c.* FROM conversations c
JOIN conversation_members cm1 ON c.id = cm1.conversation_id
JOIN conversation_members cm2 ON c.id = cm2.conversation_id
WHERE c.type = 1
  AND cm1.user_id = $1
  AND cm2.user_id = $2
LIMIT 1;

-- name: MuteGroupMember :one
INSERT INTO group_mutes (
    conversation_id,
    user_id,
    muted_until
) VALUES (
    $1, $2, $3
) ON CONFLICT (conversation_id, user_id) DO UPDATE SET
    muted_until = $3,
    created_at = NOW()
RETURNING *;

-- name: UnmuteGroupMember :exec
DELETE FROM group_mutes
WHERE conversation_id = $1 AND user_id = $2;

-- name: CheckGroupMuted :one
SELECT EXISTS (
    SELECT 1 FROM group_mutes
    WHERE conversation_id = $1 AND user_id = $2 AND muted_until > NOW()
) as is_muted;

-- name: GetGroupMuteInfo :one
SELECT * FROM group_mutes
WHERE conversation_id = $1 AND user_id = $2 AND muted_until > NOW()
LIMIT 1;
