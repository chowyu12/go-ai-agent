package mysql

import (
	"context"
	"database/sql"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/google/uuid"
)

func (s *MySQLStore) CreateFile(ctx context.Context, f *model.File) error {
	if f.UUID == "" {
		f.UUID = uuid.New().String()
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO files (uuid, conversation_id, message_id, filename, content_type, file_size, file_type, storage_path, text_content) VALUES (?,?,?,?,?,?,?,?,?)`,
		f.UUID, f.ConversationID, f.MessageID, f.Filename, f.ContentType, f.FileSize, f.FileType, f.StoragePath, sql.NullString{String: f.TextContent, Valid: f.TextContent != ""},
	)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	f.ID = id
	return nil
}

func (s *MySQLStore) GetFileByUUID(ctx context.Context, uid string) (*model.File, error) {
	var f model.File
	var textContent sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, uuid, conversation_id, message_id, filename, content_type, file_size, file_type, storage_path, text_content, created_at FROM files WHERE uuid = ?`, uid,
	).Scan(&f.ID, &f.UUID, &f.ConversationID, &f.MessageID, &f.Filename, &f.ContentType, &f.FileSize, &f.FileType, &f.StoragePath, &textContent, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	if textContent.Valid {
		f.TextContent = textContent.String
	}
	return &f, nil
}

func (s *MySQLStore) ListFilesByConversation(ctx context.Context, conversationID int64) ([]*model.File, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, conversation_id, message_id, filename, content_type, file_size, file_type, storage_path, created_at FROM files WHERE conversation_id = ? ORDER BY id`, conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (s *MySQLStore) ListFilesByMessage(ctx context.Context, messageID int64) ([]*model.File, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, uuid, conversation_id, message_id, filename, content_type, file_size, file_type, storage_path, created_at FROM files WHERE message_id = ? ORDER BY id`, messageID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFiles(rows)
}

func (s *MySQLStore) UpdateFileMessageID(ctx context.Context, fileID, messageID int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE files SET message_id = ? WHERE id = ?`, messageID, fileID)
	return err
}

func (s *MySQLStore) DeleteFile(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM files WHERE id = ?`, id)
	return err
}

func scanFiles(rows *sql.Rows) ([]*model.File, error) {
	var list []*model.File
	for rows.Next() {
		var f model.File
		if err := rows.Scan(&f.ID, &f.UUID, &f.ConversationID, &f.MessageID, &f.Filename, &f.ContentType, &f.FileSize, &f.FileType, &f.StoragePath, &f.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, &f)
	}
	return list, rows.Err()
}
