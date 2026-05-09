package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hamsaya/backend/pkg/database"
)

// AppLogEntry is one row from the app_logs table, surfaced to admins via
// /admin/logs. The `Fields` blob is decoded into an arbitrary map so the
// frontend can render structured context per entry.
type AppLogEntry struct {
	ID        string                 `json:"id"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    *string                `json:"source,omitempty"`
	RequestID *string                `json:"request_id,omitempty"`
	Error     *string                `json:"error,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type AppLogFilter struct {
	Level     string
	Search    string
	RequestID string
	Page      int
	Limit     int
}

type AppLogRepository interface {
	List(ctx context.Context, f AppLogFilter) ([]*AppLogEntry, int, error)
}

type appLogRepository struct {
	db *database.DB
}

func NewAppLogRepository(db *database.DB) AppLogRepository {
	return &appLogRepository{db: db}
}

func (r *appLogRepository) List(ctx context.Context, f AppLogFilter) ([]*AppLogEntry, int, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 200 {
		f.Limit = 50
	}
	offset := (f.Page - 1) * f.Limit

	var clauses []string
	var args []any
	if f.Level != "" {
		args = append(args, f.Level)
		clauses = append(clauses, fmt.Sprintf("level = $%d", len(args)))
	}
	if f.RequestID != "" {
		args = append(args, f.RequestID)
		clauses = append(clauses, fmt.Sprintf("request_id = $%d", len(args)))
	}
	if f.Search != "" {
		args = append(args, "%"+EscapeLike(f.Search)+"%")
		clauses = append(clauses, fmt.Sprintf(`(message ILIKE $%d ESCAPE '\' OR error ILIKE $%[1]d ESCAPE '\')`, len(args)))
	}
	where := ""
	if len(clauses) > 0 {
		where = "WHERE " + strings.Join(clauses, " AND ")
	}

	var total int
	if err := r.db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM app_logs "+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("app_logs count: %w", err)
	}

	args = append(args, f.Limit, offset)
	q := fmt.Sprintf(`
		SELECT id, level, message, source, request_id, error, fields, created_at
		FROM app_logs
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, len(args)-1, len(args))

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("app_logs list: %w", err)
	}
	defer rows.Close()

	var out []*AppLogEntry
	for rows.Next() {
		var entry AppLogEntry
		var fieldsJSON []byte
		if err := rows.Scan(
			&entry.ID, &entry.Level, &entry.Message,
			&entry.Source, &entry.RequestID, &entry.Error,
			&fieldsJSON, &entry.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("app_logs scan: %w", err)
		}
		if len(fieldsJSON) > 0 {
			_ = json.Unmarshal(fieldsJSON, &entry.Fields)
		}
		out = append(out, &entry)
	}
	return out, total, rows.Err()
}
