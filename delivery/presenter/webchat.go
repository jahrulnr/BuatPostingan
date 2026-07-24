package presenter

import (
	"time"

	"buatpostingan/internal/domain/entity"
	"buatpostingan/internal/usecase/webchat"
)

// DocsIndexGate maps entity gate → FE docs_index object.
func DocsIndexGate(g entity.DocsIndexGate) map[string]any {
	return map[string]any{
		"usable":         g.Usable,
		"status":         g.Status,
		"message":        g.Message,
		"document_count": g.DocumentCount,
	}
}

// ListConversations maps usecase result → FE list payload.
func ListConversations(out webchat.ListConversationsResult) map[string]any {
	rows := make([]map[string]any, 0, len(out.Conversations))
	for _, c := range out.Conversations {
		rows = append(rows, Conversation(c))
	}
	return map[string]any{
		"conversations": rows,
		"docs_index":    DocsIndexGate(out.DocsIndex),
	}
}

func Conversation(c webchat.ConversationView) map[string]any {
	m := c.Meta
	var title any
	if m.Title != nil {
		title = m.Title.String()
	} else {
		title = nil
	}
	return map[string]any{
		"thread_id":                m.ThreadID.String(),
		"title":                    title,
		"title_source":             string(m.TitleSource),
		"status":                   string(m.Status),
		"created_by_admin_user_id": m.CreatedByAdminUserID,
		"updated_at":               UnixMillis(m.UpdatedAt),
		"last_activity_at":         UnixMillis(m.LastActivityAt),
		"floor_holder_admin_id":    m.FloorHolderAdminID,
		"floor_remaining_sec":      c.FloorRemainingSec,
	}
}

func CreateThread(out webchat.CreateThreadResult) map[string]any {
	return map[string]any{
		"thread_id":                out.ThreadID.String(),
		"seq_head":                 out.SeqHead,
		"created_by_admin_user_id": out.CreatedByAdminUserID,
		"created_at":               out.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func RenameThread(out webchat.RenameResult) map[string]any {
	return map[string]any{
		"thread_id": out.ThreadID.String(),
		"title":     out.Title.String(),
	}
}

func StartTurn(out webchat.StartTurnResult) map[string]any {
	return map[string]any{
		"thread_id":             out.ThreadID.String(),
		"turn_id":               out.TurnID.String(),
		"seq_head":              out.SeqHead,
		"status":                out.Status,
		"floor_holder_admin_id": out.FloorHolderAdminID,
		"floor_remaining_sec":   out.FloorRemainingSec,
	}
}

func ListModels(cat entity.ModelsCatalog) map[string]any {
	rows := make([]map[string]any, 0, len(cat.Models))
	for _, m := range cat.Models {
		efforts := m.SupportedEfforts
		if efforts == nil {
			efforts = []string{}
		}
		rows = append(rows, map[string]any{
			"id":                m.ID,
			"label":             m.Label,
			"provider":          m.Provider,
			"supports_vision":   m.SupportsVision,
			"supported_efforts": efforts,
			"default_effort":    m.DefaultEffort,
			"disabled":          m.Disabled,
		})
	}
	opts := cat.EffortOptions
	if opts == nil {
		opts = []string{}
	}
	return map[string]any{
		"models":           rows,
		"default_model_id": cat.DefaultModelID,
		"stub":             cat.Stub,
		"effort": map[string]any{
			"current": cat.EffortCurrent,
			"options": opts,
		},
	}
}

func Attachment(m entity.AttachmentMeta) map[string]any {
	out := map[string]any{
		"attachment_id":             m.ID,
		"thread_id":                 m.ThreadID.String(),
		"filename":                  m.Filename,
		"mime":                      m.Mime,
		"size":                      m.Size,
		"kind":                      m.Kind,
		"uploaded_at":               m.UploadedAt.UTC().Format(time.RFC3339),
		"uploaded_by_admin_user_id": m.UploadedByID,
	}
	if m.Width > 0 {
		out["width"] = m.Width
	}
	if m.Height > 0 {
		out["height"] = m.Height
	}
	return out
}

func AttachmentList(list []entity.AttachmentMeta) map[string]any {
	rows := make([]map[string]any, 0, len(list))
	for _, m := range list {
		rows = append(rows, Attachment(m))
	}
	return map[string]any{"attachments": rows}
}

func ThreadSnapshot(snap entity.ThreadSnapshot) map[string]any {
	var activeTurn any
	if snap.ActiveTurnID != nil {
		activeTurn = snap.ActiveTurnID.String()
	}
	items := make([]map[string]any, 0, len(snap.Items))
	for _, it := range snap.Items {
		items = append(items, TranscriptItem(it))
	}
	return map[string]any{
		"thread_id":                      snap.ThreadID.String(),
		"seq_head":                       snap.SeqHead,
		"busy":                           snap.Busy,
		"floor_holder_admin_id":          snap.FloorHolderAdminID,
		"floor_remaining_sec":            snap.FloorRemainingSec,
		"active_turn_id":                 activeTurn,
		"active_turn_initiator_admin_id": snap.ActiveTurnInitiatorAdminID,
		"items":                          items,
	}
}

// TranscriptItem flattens Payload into the FE item object.
func TranscriptItem(it entity.TranscriptItem) map[string]any {
	out := map[string]any{
		"seq":       it.Seq,
		"id":        it.ID.String(),
		"thread_id": it.ThreadID.String(),
		"type":      string(it.Type),
	}
	if it.TurnID.String() != "" {
		out["turn_id"] = it.TurnID.String()
	}
	for k, v := range it.Payload {
		if _, exists := out[k]; exists {
			continue
		}
		out[k] = v
	}
	return out
}
