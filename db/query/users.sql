-- name: CreateUser :one
INSERT INTO users (
  username,
  passwd_hash
) VALUES (
  $1, $2
) RETURNING *;

-- name: GetUserById :one
SELECT * FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT * FROM users
WHERE username = $1 LIMIT 1;


-- name: UpdatePasswd :one
UPDATE users
SET passwd_hash = $2
WHERE id = $1
RETURNING *;

-- name: UpdateUsername :one
UPDATE users
SET username = $2
WHERE id = $1
RETURNING *;

-- name: UpdataStatus :one
UPDATE users
SET status = $2
WHERE id = $1
RETURNING *;

-- name: UpdateAvatar :one
UPDATE users
SET avatar_url = $2
WHERE id = $1
RETURNING *;
