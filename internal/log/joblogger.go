package log

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jackc/pgtype"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/util"
)

type JobLogger struct {
	store *repository.Store
}

type Level uint8

const (
	Emergency Level = iota
	Alert
	Critical
	Error
	Warning
	Notice
	Info
	Debug
)

func (l Level) String() string {
	switch l {
	case Emergency:
		return "emergency"
	case Alert:
		return "alert"
	case Critical:
		return "critical"
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Notice:
		return "notice"
	case Info:
		return "info"
	case Debug:
		return "debug"
	}

	return ""
}

type JobLog struct {
	ID        int64
	CreatedAt time.Time
	Level     int16
	JobID     string
	Message   string
	Metadata  map[string]any
}

func NewJobLogger(store *repository.Store) *JobLogger {
	return &JobLogger{store}
}

func (l *JobLogger) Log(ctx context.Context, level Level, jobID, msg string, metadata map[string]any) {
	metadataRaw := pgtype.JSONB{Status: pgtype.Null}

	if len(metadata) > 0 {
		bytes, _ := json.Marshal(metadata)
		metadataRaw.Bytes = bytes
		metadataRaw.Status = pgtype.Present
	}

	_ = l.store.CreateJobLog(ctx, repository.CreateJobLogParams{
		CreatedAt: time.Now(),
		Level:     int16(level),
		JobID:     sql.NullString{String: jobID, Valid: true},
		Message:   msg,
		Metadata:  metadataRaw,
	})
}

func (l *JobLogger) ListByJobID(ctx context.Context, jobID string, limit int64) ([]*JobLog, error) {
	entries, err := l.store.ListJobLogsByID(ctx, repository.ListJobLogsByIDParams{
		JobID: repository.StringToNullable(jobID),
		Limit: int32(limit),
	})

	if err != nil {
		return nil, err
	}

	return util.MapSlice(entries, entryToJobLog), nil
}

func entryToJobLog(e repository.JobLog) *JobLog {
	metadata := make(map[string]any)
	_ = json.Unmarshal(e.Metadata.Bytes, &metadata)

	return &JobLog{
		ID:        e.ID,
		CreatedAt: e.CreatedAt,
		Level:     e.Level,
		JobID:     e.JobID.String,
		Message:   e.Message,
		Metadata:  metadata,
	}
}
