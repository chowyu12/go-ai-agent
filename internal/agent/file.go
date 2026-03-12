package agent

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/chowyu12/go-ai-agent/internal/model"
	"github.com/chowyu12/go-ai-agent/internal/prompt"
)

func (e *Executor) loadRequestFiles(ctx context.Context, chatFiles []model.ChatFile, conversationID int64) []*model.File {
	var files []*model.File
	seen := make(map[string]bool)

	for _, cf := range chatFiles {
		switch cf.TransferMethod {
		case model.TransferLocalFile:
			if cf.UploadFileID == "" {
				continue
			}
			f, err := e.store.GetFileByUUID(ctx, cf.UploadFileID)
			if err != nil {
				log.WithField("upload_file_id", cf.UploadFileID).WithError(err).Warn("[Prepare] load uploaded file failed, skipping")
				continue
			}
			seen[f.UUID] = true
			files = append(files, f)
		case model.TransferRemoteURL:
			if cf.URL == "" {
				continue
			}
			if seen[cf.URL] {
				continue
			}
			f := prompt.LoadRemoteFile(ctx, cf.URL, cf.Type)
			if f != nil {
				seen[cf.URL] = true
				files = append(files, f)
			}
		}
	}

	if conversationID > 0 {
		convFiles, err := e.store.ListFilesByConversation(ctx, conversationID)
		if err == nil {
			for _, f := range convFiles {
				if !seen[f.UUID] {
					seen[f.UUID] = true
					files = append(files, f)
				}
			}
		}
	}

	if len(files) > 0 {
		names := make([]string, 0, len(files))
		for _, f := range files {
			names = append(names, fmt.Sprintf("%s(%s)", f.Filename, f.FileType))
		}
		log.WithField("files", names).Info("[Prepare] files loaded for context")
	}
	return files
}
