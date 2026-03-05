package model

import "time"

type FileType string

const (
	FileTypeText     FileType = "text"
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
)

type File struct {
	ID             int64     `json:"id"`
	UUID           string    `json:"uuid"`
	ConversationID int64     `json:"conversation_id"`
	MessageID      int64     `json:"message_id"`
	Filename       string    `json:"filename"`
	ContentType    string    `json:"content_type"`
	FileSize       int64     `json:"file_size"`
	FileType       FileType  `json:"file_type"`
	StoragePath    string    `json:"-"`
	TextContent    string    `json:"text_content,omitzero"`
	CreatedAt      time.Time `json:"created_at"`
}

func (f *File) IsImage() bool {
	return f.FileType == FileTypeImage
}

func (f *File) IsTextual() bool {
	return f.FileType == FileTypeText || f.FileType == FileTypeDocument
}
