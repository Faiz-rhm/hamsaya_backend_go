package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/hamsaya/backend/internal/models"
	"github.com/hamsaya/backend/pkg/database"
)

type CustomRoleRepository interface {
	List(ctx context.Context) ([]*models.CustomRole, error)
	Get(ctx context.Context, id string) (*models.CustomRole, error)
	GetByName(ctx context.Context, name string) (*models.CustomRole, error)
	Create(ctx context.Context, req *models.CreateCustomRoleRequest, createdBy string) (*models.CustomRole, error)
	Update(ctx context.Context, id string, req *models.UpdateCustomRoleRequest, updatedBy string) (*models.CustomRole, error)
	Delete(ctx context.Context, id string) error
	// Assign sets or clears a user's custom role.
	Assign(ctx context.Context, userID string, customRoleID *string) error
	// ListUsers returns admin users assigned to a specific custom role.
	ListUsers(ctx context.Context, customRoleID string) ([]*models.CustomRoleUser, error)
	// GetUserCustomRole returns the custom role assigned to a user, nil if none.
	GetUserCustomRole(ctx context.Context, userID string) (*models.CustomRole, error)
}

type customRoleRepository struct {
	db *database.DB
}

func NewCustomRoleRepository(db *database.DB) CustomRoleRepository {
	return &customRoleRepository{db: db}
}

func (r *customRoleRepository) scan(row pgx.Row) (*models.CustomRole, error) {
	cr := &models.CustomRole{}
	var permsJSON []byte
	var userCount *int
	err := row.Scan(
		&cr.ID, &cr.Name, &cr.Description, &permsJSON,
		&cr.CreatedBy, &cr.UpdatedBy, &cr.CreatedAt, &cr.UpdatedAt, &userCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("custom_role scan: %w", err)
	}
	if len(permsJSON) > 0 {
		_ = json.Unmarshal(permsJSON, &cr.Permissions)
	}
	if cr.Permissions == nil {
		cr.Permissions = []string{}
	}
	if userCount != nil {
		cr.UserCount = *userCount
	}
	return cr, nil
}

const selectCR = `
	SELECT cr.id, cr.name, cr.description, cr.permissions,
	       cr.created_by::text, cr.updated_by::text, cr.created_at, cr.updated_at,
	       (SELECT COUNT(*) FROM users u WHERE u.custom_role_id = cr.id)::int AS user_count
	FROM custom_roles cr
`

func (r *customRoleRepository) List(ctx context.Context) ([]*models.CustomRole, error) {
	rows, err := r.db.Pool.Query(ctx, selectCR+" ORDER BY cr.name ASC")
	if err != nil {
		return nil, fmt.Errorf("custom_roles list: %w", err)
	}
	defer rows.Close()

	var out []*models.CustomRole
	for rows.Next() {
		cr, err := r.scan(rows)
		if err != nil {
			return nil, err
		}
		if cr != nil {
			out = append(out, cr)
		}
	}
	if out == nil {
		out = []*models.CustomRole{}
	}
	return out, rows.Err()
}

func (r *customRoleRepository) Get(ctx context.Context, id string) (*models.CustomRole, error) {
	return r.scan(r.db.Pool.QueryRow(ctx, selectCR+" WHERE cr.id = $1", id))
}

func (r *customRoleRepository) GetByName(ctx context.Context, name string) (*models.CustomRole, error) {
	return r.scan(r.db.Pool.QueryRow(ctx, selectCR+" WHERE cr.name = $1", name))
}

func (r *customRoleRepository) Create(
	ctx context.Context,
	req *models.CreateCustomRoleRequest,
	createdBy string,
) (*models.CustomRole, error) {
	permsJSON, _ := json.Marshal(req.Permissions)
	desc := ""
	if req.Description != nil {
		desc = *req.Description
	}
	var id string
	err := r.db.Pool.QueryRow(ctx, `
		INSERT INTO custom_roles (name, description, permissions, created_by, updated_by)
		VALUES ($1, NULLIF($2,''), $3, $4, $4)
		RETURNING id
	`, req.Name, desc, permsJSON, createdBy).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("custom_role create: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *customRoleRepository) Update(
	ctx context.Context,
	id string,
	req *models.UpdateCustomRoleRequest,
	updatedBy string,
) (*models.CustomRole, error) {
	current, err := r.Get(ctx, id)
	if err != nil || current == nil {
		return nil, err
	}
	name := current.Name
	if req.Name != nil {
		name = *req.Name
	}
	var desc *string
	if req.Description != nil {
		desc = req.Description
	} else {
		desc = current.Description
	}
	perms := current.Permissions
	if req.Permissions != nil {
		perms = req.Permissions
	}
	permsJSON, _ := json.Marshal(perms)
	_, err = r.db.Pool.Exec(ctx, `
		UPDATE custom_roles
		SET name=$2, description=$3, permissions=$4, updated_by=$5, updated_at=$6
		WHERE id=$1
	`, id, name, desc, permsJSON, updatedBy, time.Now())
	if err != nil {
		return nil, fmt.Errorf("custom_role update: %w", err)
	}
	return r.Get(ctx, id)
}

func (r *customRoleRepository) Delete(ctx context.Context, id string) error {
	// Clear assignments before deleting — ON DELETE SET NULL handles this via FK.
	_, err := r.db.Pool.Exec(ctx, `DELETE FROM custom_roles WHERE id = $1`, id)
	return err
}

func (r *customRoleRepository) Assign(ctx context.Context, userID string, customRoleID *string) error {
	_, err := r.db.Pool.Exec(ctx,
		`UPDATE users SET custom_role_id = $2 WHERE id = $1`,
		userID, customRoleID)
	return err
}

func (r *customRoleRepository) ListUsers(ctx context.Context, customRoleID string) ([]*models.CustomRoleUser, error) {
	rows, err := r.db.Pool.Query(ctx, `
		SELECT id, email, role, created_at
		FROM users
		WHERE custom_role_id = $1
		ORDER BY email
	`, customRoleID)
	if err != nil {
		return nil, fmt.Errorf("custom_role users: %w", err)
	}
	defer rows.Close()
	var out []*models.CustomRoleUser
	for rows.Next() {
		u := &models.CustomRoleUser{}
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.Role, &createdAt); err != nil {
			return nil, fmt.Errorf("custom_role user scan: %w", err)
		}
		u.CreatedAt = createdAt.Format(time.RFC3339)
		out = append(out, u)
	}
	if out == nil {
		out = []*models.CustomRoleUser{}
	}
	return out, rows.Err()
}

func (r *customRoleRepository) GetUserCustomRole(ctx context.Context, userID string) (*models.CustomRole, error) {
	return r.scan(r.db.Pool.QueryRow(ctx, selectCR+`
		JOIN users u ON u.custom_role_id = cr.id
		WHERE u.id = $1
	`, userID))
}
