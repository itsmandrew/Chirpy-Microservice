-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(), NOW(), NOW(), $1, $2
)
RETURNING id, created_at, updated_at, email;

-- name: DeleteUsers :exec
TRUNCATE TABLE users CASCADE;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;


-- name: UpdateUserPassword :exec
UPDATE users
    SET hashed_password = $1,
        email = $2,
        updated_at = NOW()
WHERE id = $3;


-- name: GetUserByIDNoPassword :one
SELECT id, created_at, updated_at, email
FROM users
WHERE id = $1;