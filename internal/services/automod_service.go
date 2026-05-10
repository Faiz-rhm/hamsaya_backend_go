package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/hamsaya/backend/pkg/database"
)

// AutomodService runs every post-create through the active rule set and
// reports the strictest matching action. Rules are cached in-process for
// 30 seconds; admin CRUD invalidates the cache so changes apply quickly
// without a restart.
type AutomodService struct {
	db     *database.DB
	logger *zap.Logger

	mu       sync.RWMutex
	cache    []compiledRule
	cachedAt time.Time
	ttl      time.Duration
}

func NewAutomodService(db *database.DB, logger *zap.Logger) *AutomodService {
	return &AutomodService{
		db:     db,
		logger: logger,
		ttl:    30 * time.Second,
	}
}

// AutomodRule mirrors a row in automod_rules.
type AutomodRule struct {
	ID          uuid.UUID  `json:"id"`
	Pattern     string     `json:"pattern"`
	IsRegex     bool       `json:"is_regex"`
	Action      string     `json:"action"`
	Severity    string     `json:"severity"`
	Enabled     bool       `json:"enabled"`
	Description *string    `json:"description,omitempty"`
	CreatedBy   *string    `json:"created_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	LastHitAt   *time.Time `json:"last_hit_at,omitempty"`
	HitCount    int64      `json:"hit_count"`
}

// AutomodMatch is the result of a content scan. Action is empty when
// nothing fired.
type AutomodMatch struct {
	RuleID      uuid.UUID `json:"rule_id"`
	Pattern     string    `json:"pattern"`
	Action      string    `json:"action"`   // block | flag | shadow
	Severity    string    `json:"severity"` // low | medium | high | critical
	Description string    `json:"description"`
}

type compiledRule struct {
	row     AutomodRule
	regex   *regexp.Regexp // nil when is_regex=false
	literal string         // lowercased for substring scans
}

// ErrAutomodBlocked is returned by Scan when a 'block' rule matches.
// Callers in the post-create path should surface this as a 422 with the
// rule description so the user knows why their post was rejected.
var ErrAutomodBlocked = errors.New("post blocked by automod rule")

// Scan checks `text` against the active rule set. Returns the
// highest-severity match, or zero-value match when nothing fires. Errors
// from the rule load are returned to the caller — fail-open vs.
// fail-closed is a policy decision left to the post-create handler.
func (s *AutomodService) Scan(ctx context.Context, text string) (AutomodMatch, error) {
	rules, err := s.cachedRules(ctx)
	if err != nil {
		return AutomodMatch{}, err
	}
	match := scanCompiledRules(rules, text)
	if match.RuleID != uuid.Nil {
		// Increment hit_count async; failure is non-fatal.
		go s.recordHit(match.RuleID)
	}
	return match, nil
}

// scanCompiledRules is the pure-logic scanner extracted for unit
// testing. Walks the rule set, picks the strictest match. block beats
// flag/shadow; within same action, higher severity wins.
func scanCompiledRules(rules []compiledRule, text string) AutomodMatch {
	if len(rules) == 0 {
		return AutomodMatch{}
	}

	lower := strings.ToLower(text)
	rank := map[string]int{"low": 0, "medium": 1, "high": 2, "critical": 3}

	var best AutomodMatch
	bestScore := -1

	for _, r := range rules {
		match := false
		if r.regex != nil {
			match = r.regex.MatchString(text)
		} else {
			match = strings.Contains(lower, r.literal)
		}
		if !match {
			continue
		}
		score := rank[r.row.Severity]
		if r.row.Action == "block" {
			score += 100
		}
		if score > bestScore {
			bestScore = score
			desc := ""
			if r.row.Description != nil {
				desc = *r.row.Description
			}
			best = AutomodMatch{
				RuleID:      r.row.ID,
				Pattern:     r.row.Pattern,
				Action:      r.row.Action,
				Severity:    r.row.Severity,
				Description: desc,
			}
		}
	}
	return best
}

// compileRule converts an AutomodRule (DB row) into a compiledRule
// (with prepared regex / lowercase literal). Extracted for tests.
func compileRule(r AutomodRule) (compiledRule, error) {
	c := compiledRule{row: r, literal: strings.ToLower(r.Pattern)}
	if r.IsRegex {
		rx, err := regexp.Compile("(?i)" + r.Pattern)
		if err != nil {
			return compiledRule{}, err
		}
		c.regex = rx
	}
	return c, nil
}

// recordHit bumps hit_count + last_hit_at. Best-effort; we don't want a
// stats update to slow the post-create hot path.
func (s *AutomodService) recordHit(ruleID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := s.db.Pool.Exec(ctx,
		`UPDATE automod_rules SET hit_count = hit_count + 1, last_hit_at = NOW() WHERE id = $1`,
		ruleID,
	); err != nil {
		s.logger.Warn("automod hit count update failed", zap.String("rule_id", ruleID.String()), zap.Error(err))
	}
}

func (s *AutomodService) cachedRules(ctx context.Context) ([]compiledRule, error) {
	s.mu.RLock()
	if time.Since(s.cachedAt) < s.ttl && s.cache != nil {
		out := s.cache
		s.mu.RUnlock()
		return out, nil
	}
	s.mu.RUnlock()

	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, pattern, is_regex, action, severity, enabled, description,
		       created_by::text, created_at, updated_at, last_hit_at, hit_count
		FROM automod_rules
		WHERE enabled = TRUE
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]compiledRule, 0)
	for rows.Next() {
		var r AutomodRule
		var createdBy string
		if err := rows.Scan(&r.ID, &r.Pattern, &r.IsRegex, &r.Action, &r.Severity,
			&r.Enabled, &r.Description, &createdBy, &r.CreatedAt, &r.UpdatedAt,
			&r.LastHitAt, &r.HitCount,
		); err != nil {
			return nil, err
		}
		if createdBy != "" {
			r.CreatedBy = &createdBy
		}
		c := compiledRule{row: r, literal: strings.ToLower(r.Pattern)}
		if r.IsRegex {
			rx, err := regexp.Compile("(?i)" + r.Pattern)
			if err != nil {
				s.logger.Warn("automod rule regex invalid; skipping",
					zap.String("rule_id", r.ID.String()), zap.String("pattern", r.Pattern), zap.Error(err))
				continue
			}
			c.regex = rx
		}
		out = append(out, c)
	}

	s.mu.Lock()
	s.cache = out
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return out, nil
}

func (s *AutomodService) invalidateCache() {
	s.mu.Lock()
	s.cache = nil
	s.cachedAt = time.Time{}
	s.mu.Unlock()
}

// ─── Admin CRUD ──────────────────────────────────────────────────────────────

func (s *AutomodService) List(ctx context.Context) ([]AutomodRule, error) {
	rows, err := s.db.Pool.Query(ctx, `
		SELECT id, pattern, is_regex, action, severity, enabled, description,
		       created_by::text, created_at, updated_at, last_hit_at, hit_count
		FROM automod_rules
		ORDER BY enabled DESC, severity DESC, created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AutomodRule, 0)
	for rows.Next() {
		var r AutomodRule
		var createdBy string
		if err := rows.Scan(&r.ID, &r.Pattern, &r.IsRegex, &r.Action, &r.Severity,
			&r.Enabled, &r.Description, &createdBy, &r.CreatedAt, &r.UpdatedAt,
			&r.LastHitAt, &r.HitCount,
		); err != nil {
			return nil, err
		}
		if createdBy != "" {
			r.CreatedBy = &createdBy
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *AutomodService) Create(ctx context.Context, pattern string, isRegex bool, action, severity, description, adminID string) (uuid.UUID, error) {
	if err := validateRuleFields(pattern, action, severity, isRegex); err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	if _, err := s.db.Pool.Exec(ctx, `
		INSERT INTO automod_rules (id, pattern, is_regex, action, severity, description, created_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), $7)
	`, id, pattern, isRegex, action, severity, description, adminID); err != nil {
		return uuid.Nil, err
	}
	s.invalidateCache()
	return id, nil
}

func (s *AutomodService) Update(ctx context.Context, id, pattern string, isRegex bool, action, severity, description string, enabled bool) error {
	if err := validateRuleFields(pattern, action, severity, isRegex); err != nil {
		return err
	}
	tag, err := s.db.Pool.Exec(ctx, `
		UPDATE automod_rules
		SET pattern=$1, is_regex=$2, action=$3, severity=$4, description=NULLIF($5,''), enabled=$6, updated_at=NOW()
		WHERE id=$7
	`, pattern, isRegex, action, severity, description, enabled, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("rule %s not found", id)
	}
	s.invalidateCache()
	return nil
}

func (s *AutomodService) Delete(ctx context.Context, id string) error {
	tag, err := s.db.Pool.Exec(ctx, `DELETE FROM automod_rules WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("rule %s not found", id)
	}
	s.invalidateCache()
	return nil
}

func validateRuleFields(pattern, action, severity string, isRegex bool) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("pattern required")
	}
	if isRegex {
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex: %w", err)
		}
	}
	switch action {
	case "block", "flag", "shadow":
	default:
		return fmt.Errorf("invalid action %q (allowed: block, flag, shadow)", action)
	}
	switch severity {
	case "low", "medium", "high", "critical":
	default:
		return fmt.Errorf("invalid severity %q (allowed: low, medium, high, critical)", severity)
	}
	return nil
}
