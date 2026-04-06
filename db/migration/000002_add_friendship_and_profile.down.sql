-- 删除新增的表（按依赖倒序）
DROP TABLE IF EXISTS "group_announcements" CASCADE;
DROP TABLE IF EXISTS "message_forwards" CASCADE;
DROP TABLE IF EXISTS "user_profiles" CASCADE;
DROP TABLE IF EXISTS "friendships" CASCADE;
