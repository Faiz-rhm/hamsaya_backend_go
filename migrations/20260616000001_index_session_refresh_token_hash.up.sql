-- Index the session refresh-token-hash lookup.
--
-- `refresh_token_hash` was added in 20260208000001 but never indexed, so the
-- hot-path lookup `SELECT ... FROM user_sessions WHERE refresh_token_hash = $1`
-- (run on every token refresh) did a full sequential scan — observed at ~2.8s
-- under production load. A plain btree on the hash makes it an index lookup.
--
-- NOT partial: the query carries no `revoked` predicate, so a partial index
-- (WHERE revoked = false) would not be eligible for the planner. Keep it full.
CREATE INDEX IF NOT EXISTS idx_user_sessions_refresh_token_hash
    ON user_sessions (refresh_token_hash);
