package observability

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/hamsaya/backend/pkg/database"
)

// DBLogSink is a zapcore.Core that mirrors warn+ entries to the app_logs
// table. It runs in a background goroutine driven by a buffered channel so
// the hot path stays in-memory; backpressure drops oldest entries rather
// than blocking the application logger.
//
// The sink is intentionally additive — pair it with a tee'd Core that also
// writes to stdout so container logs continue to flow into the existing
// Loki/Grafana pipeline.
type DBLogSink struct {
	level zapcore.LevelEnabler
	enc   zapcore.Encoder
	db    *database.DB
	ch    chan *bufferedEntry
	once  sync.Once
}

type bufferedEntry struct {
	entry  zapcore.Entry
	fields []zapcore.Field
	ctx    []zapcore.Field // accumulated With(...) fields
}

// NewDBLogSink wires a sink for `level` and above. `bufferSize` bounds the
// channel; oversize bursts evict oldest. Caller is responsible for calling
// `Start` once the database is ready.
func NewDBLogSink(db *database.DB, level zapcore.Level, bufferSize int) *DBLogSink {
	cfg := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "level",
		TimeKey:       "ts",
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder,
		EncodeCaller:  zapcore.ShortCallerEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
	}
	if bufferSize <= 0 {
		bufferSize = 256
	}
	return &DBLogSink{
		level: level,
		enc:   zapcore.NewJSONEncoder(cfg),
		db:    db,
		ch:    make(chan *bufferedEntry, bufferSize),
	}
}

// zapcore.Core implementation.

func (s *DBLogSink) Enabled(l zapcore.Level) bool { return s.level.Enabled(l) }

func (s *DBLogSink) With(fields []zapcore.Field) zapcore.Core {
	clone := &DBLogSink{
		level: s.level,
		enc:   s.enc,
		db:    s.db,
		ch:    s.ch,
	}
	// We don't actually maintain an accumulated context here; zap clones via
	// the wrapping logger, which calls Write with the union of fields.
	_ = fields
	return clone
}

func (s *DBLogSink) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if s.Enabled(ent.Level) {
		return ce.AddCore(ent, s)
	}
	return ce
}

func (s *DBLogSink) Write(ent zapcore.Entry, fields []zapcore.Field) error {
	// Non-blocking enqueue. Drop oldest if full so the logger never stalls.
	be := &bufferedEntry{entry: ent, fields: fields}
	select {
	case s.ch <- be:
	default:
		select {
		case <-s.ch:
		default:
		}
		select {
		case s.ch <- be:
		default:
		}
	}
	return nil
}

func (s *DBLogSink) Sync() error { return nil }

// Start launches the background drainer. Safe to call multiple times.
func (s *DBLogSink) Start(ctx context.Context) {
	s.once.Do(func() {
		go s.drain(ctx)
	})
}

func (s *DBLogSink) drain(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case be := <-s.ch:
			if be == nil {
				continue
			}
			s.persist(ctx, be)
		}
	}
}

func (s *DBLogSink) persist(ctx context.Context, be *bufferedEntry) {
	if s.db == nil || s.db.Pool == nil {
		return
	}

	enc := zapcore.NewMapObjectEncoder()
	for _, f := range be.fields {
		f.AddTo(enc)
	}

	requestID, _ := enc.Fields["request_id"].(string)
	if requestID == "" {
		requestID, _ = enc.Fields["X-Request-Id"].(string)
	}
	errStr, _ := enc.Fields["error"].(string)

	// Strip request_id and error from the structured-fields blob so the
	// dedicated columns aren't duplicated.
	delete(enc.Fields, "request_id")
	delete(enc.Fields, "X-Request-Id")
	delete(enc.Fields, "error")

	var fieldsJSON []byte
	if len(enc.Fields) > 0 {
		fieldsJSON, _ = json.Marshal(enc.Fields)
	}

	insertCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, _ = s.db.Pool.Exec(insertCtx, `
		INSERT INTO app_logs (level, message, source, request_id, error, fields, created_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), $6, $7)
	`,
		be.entry.Level.String(),
		be.entry.Message,
		be.entry.LoggerName,
		requestID,
		errStr,
		fieldsJSON,
		be.entry.Time,
	)
}
