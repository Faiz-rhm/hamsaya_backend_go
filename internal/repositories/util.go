package repositories

import "strings"

// likeReplacer escapes the SQL LIKE/ILIKE wildcards `%` and `_` and the
// escape character `\` itself. Pair the result with `ESCAPE '\'` in the SQL
// statement so user-supplied wildcards are matched literally.
var likeReplacer = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// EscapeLike escapes user input before substring-matching with LIKE/ILIKE.
// Caller must append `ESCAPE '\'` after the LIKE/ILIKE clause.
func EscapeLike(s string) string {
	return likeReplacer.Replace(s)
}
