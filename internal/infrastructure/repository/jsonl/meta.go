package jsonl

import (
	"encoding/json"
	"os"
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/domain/enum"
	"buatpostingan/internal/domain/valueobject"
)

type sessionMetaRow struct {
	ThreadID                   string  `json:"thread_id"`
	CreatedByAdminUserID       int64   `json:"created_by_admin_user_id"`
	AdminUserID                int64   `json:"admin_user_id"`
	Title                      *string `json:"title"`
	TitleSource                string  `json:"title_source"`
	Status                     string  `json:"status"`
	Path                       string  `json:"path"`
	UpdatedAt                  float64 `json:"updated_at"`
	LastActivityAt             float64 `json:"last_activity_at"`
	FloorHolderAdminID         *int64  `json:"floor_holder_admin_id"`
	FloorLastTurnAt            *float64 `json:"floor_last_turn_at"`
	ActiveTurnID               *string `json:"active_turn_id"`
	ActiveTurnInitiatorAdminID *int64  `json:"active_turn_initiator_admin_id"`
}

func timeToUnixFloat(t time.Time) float64 {
	if t.IsZero() {
		return 0
	}
	return float64(t.UnixNano()) / 1e9
}

func unixFloatToTime(v float64) time.Time {
	if v <= 0 {
		return time.Time{}
	}
	sec := int64(v)
	nsec := int64((v - float64(sec)) * 1e9)
	return time.Unix(sec, nsec).UTC()
}

func metaToRow(meta entity.ConversationMeta, path string) sessionMetaRow {
	row := sessionMetaRow{
		ThreadID:                   meta.ThreadID.String(),
		CreatedByAdminUserID:       meta.CreatedByAdminUserID,
		AdminUserID:                meta.CreatedByAdminUserID,
		TitleSource:                string(meta.TitleSource),
		Status:                     string(meta.Status),
		Path:                       path,
		UpdatedAt:                  timeToUnixFloat(meta.UpdatedAt),
		LastActivityAt:             timeToUnixFloat(meta.LastActivityAt),
		FloorHolderAdminID:         meta.FloorHolderAdminID,
		ActiveTurnInitiatorAdminID: meta.ActiveTurnInitiatorAdminID,
	}
	if meta.Title != nil {
		t := meta.Title.String()
		row.Title = &t
	}
	if meta.FloorLastTurnAt != nil {
		f := timeToUnixFloat(*meta.FloorLastTurnAt)
		row.FloorLastTurnAt = &f
	}
	if meta.ActiveTurnID != nil {
		s := meta.ActiveTurnID.String()
		row.ActiveTurnID = &s
	}
	if row.TitleSource == "" {
		row.TitleSource = string(enum.TitlePending)
	}
	if row.Status == "" {
		row.Status = string(enum.ConversationActive)
	}
	return row
}

func rowToMeta(row sessionMetaRow) (entity.ConversationMeta, error) {
	tid, err := valueobject.NewThreadID(row.ThreadID)
	if err != nil {
		return entity.ConversationMeta{}, err
	}
	meta := entity.ConversationMeta{
		ThreadID:                   tid,
		TitleSource:                enum.TitleSource(row.TitleSource),
		Status:                     enum.ConversationStatus(row.Status),
		CreatedByAdminUserID:       row.CreatedByAdminUserID,
		UpdatedAt:                  unixFloatToTime(row.UpdatedAt),
		LastActivityAt:             unixFloatToTime(row.LastActivityAt),
		FloorHolderAdminID:         row.FloorHolderAdminID,
		ActiveTurnInitiatorAdminID: row.ActiveTurnInitiatorAdminID,
	}
	if meta.CreatedByAdminUserID == 0 && row.AdminUserID != 0 {
		meta.CreatedByAdminUserID = row.AdminUserID
	}
	if meta.TitleSource == "" {
		meta.TitleSource = enum.TitlePending
	}
	if meta.Status == "" {
		meta.Status = enum.ConversationActive
	}
	if row.Title != nil && *row.Title != "" {
		title, terr := valueobject.NewTitle(*row.Title)
		if terr == nil {
			meta.Title = &title
		}
	}
	if row.FloorLastTurnAt != nil {
		t := unixFloatToTime(*row.FloorLastTurnAt)
		meta.FloorLastTurnAt = &t
	}
	if row.ActiveTurnID != nil && *row.ActiveTurnID != "" {
		turn, terr := valueobject.NewTurnID(*row.ActiveTurnID)
		if terr == nil {
			meta.ActiveTurnID = &turn
		}
	}
	return meta, nil
}

func appendJSONLLine(path string, v any) error {
	if err := os.MkdirAll(dirOf(path), 0o775); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o664)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := flockExclusive(f); err != nil {
		return err
	}
	defer flockUnlock(f)
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(v) // Encode adds trailing newline
}

// readSessionIndexLines scans line-by-line so one corrupt line does not stop the rest.
func readSessionIndexLines(path string) ([]sessionMetaRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var rows []sessionMetaRow
	start := 0
	for i := 0; i <= len(raw); i++ {
		if i < len(raw) && raw[i] != '\n' {
			continue
		}
		line := raw[start:i]
		start = i + 1
		if len(line) == 0 || (len(line) == 1 && line[0] == '\r') {
			continue
		}
		var row sessionMetaRow
		if err := json.Unmarshal(line, &row); err != nil || row.ThreadID == "" {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}
