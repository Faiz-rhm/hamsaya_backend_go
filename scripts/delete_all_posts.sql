-- Delete all posts and related data (respects foreign keys order)
DELETE FROM comment_attachments;
DELETE FROM comment_reports;
DELETE FROM post_comment_likes;
DELETE FROM post_comments;
DELETE FROM post_reports;
DELETE FROM post_bookmarks;
DELETE FROM post_likes;
DELETE FROM post_shares;
DELETE FROM event_interests;
DELETE FROM user_polls;
DELETE FROM poll_options;
DELETE FROM polls;
DELETE FROM attachments;
DELETE FROM posts;
