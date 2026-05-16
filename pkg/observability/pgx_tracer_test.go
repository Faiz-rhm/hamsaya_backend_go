package observability

import "testing"

func TestClassifySQL(t *testing.T) {
	tests := []struct {
		name    string
		sql     string
		wantOp  string
		wantTbl string
	}{
		{"select with from", "SELECT id, name FROM users WHERE id = $1", "SELECT", "users"},
		{"select lowercase", "select * from posts where deleted_at is null", "SELECT", "posts"},
		{"select with schema-qualified table", "SELECT * FROM public.businesses WHERE id=$1", "SELECT", "public.businesses"},
		{"insert into", "INSERT INTO post_comments (id, text) VALUES ($1, $2)", "INSERT", "post_comments"},
		{"update", "UPDATE users SET email_verified=true WHERE id=$1", "UPDATE", "users"},
		{"delete from", "DELETE FROM sessions WHERE expires_at < NOW()", "DELETE", "sessions"},
		{"cte", "WITH ranked AS (SELECT id FROM posts) SELECT * FROM ranked", "CTE", ""},
		{"tx begin", "BEGIN", "TX", ""},
		{"tx commit", "COMMIT", "TX", ""},
		{"tx rollback", "ROLLBACK", "TX", ""},
		{"empty sql", "", "OTHER", ""},
		{"unknown", "VACUUM ANALYZE users", "OTHER", ""},
		{"leading whitespace", "   SELECT 1", "SELECT", ""}, // no FROM clause
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, tbl := classifySQL(tt.sql)
			if op != tt.wantOp {
				t.Errorf("operation: got %q, want %q", op, tt.wantOp)
			}
			if tbl != tt.wantTbl {
				t.Errorf("table: got %q, want %q", tbl, tt.wantTbl)
			}
		})
	}
}

// firstToken only accepts uppercase letters because production callers
// uppercase the SQL before slicing into it. These tests mirror that
// contract so the cases match the upper-case substrings classifySQL
// actually feeds in.
func TestFirstToken(t *testing.T) {
	tests := map[string]string{
		"USERS":               "users",
		"USERS WHERE ID=1":    "users",
		"  POSTS ORDER BY ID": "posts",
		"PUBLIC.BUSINESSES":   "public.businesses",
		"":                    "",
		"\n\n   ":             "",
		"!@#$%":               "",
	}
	for input, want := range tests {
		if got := firstToken(input); got != want {
			t.Errorf("firstToken(%q): got %q, want %q", input, got, want)
		}
	}
}
