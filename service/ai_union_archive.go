package service

import (
	"context"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	AIUnionTaskAction          = "ai_union_video"
	aiUnionSourceURLTTLSeconds = 24 * 60 * 60
)

type AIUnionMediaURLs struct {
	VideoURL     string
	LastFrameURL string
}

func ExtractAIUnionMediaURLsFromTask(task *model.Task) AIUnionMediaURLs {
	if task == nil {
		return AIUnionMediaURLs{}
	}
	urls := AIUnionMediaURLs{
		VideoURL: strings.TrimSpace(task.GetResultURL()),
	}
	var data any
	if len(task.Data) > 0 && common.Unmarshal(task.Data, &data) == nil {
		if videoURL := findStringValue(data, "video_url"); videoURL != "" {
			urls.VideoURL = videoURL
		}
		urls.LastFrameURL = findStringValue(data, "last_frame_url")
	}
	return urls
}

func EnsureAIUnionMediaRecordsForTask(task *model.Task, urls AIUnionMediaURLs) ([]*model.AiUnionMedia, error) {
	if task == nil || task.UserId == 0 || task.TaskID == "" {
		return nil, nil
	}

	now := time.Now().Unix()
	expiresAt := now + aiUnionSourceURLTTLSeconds
	records := make([]*model.AiUnionMedia, 0, 2)

	if urls.VideoURL != "" {
		media, err := ensureAIUnionMediaRecord(task, model.AiUnionMediaKindVideo, urls.VideoURL, expiresAt, now)
		if err != nil {
			return nil, err
		}
		records = append(records, media)
	}
	if urls.LastFrameURL != "" {
		media, err := ensureAIUnionMediaRecord(task, model.AiUnionMediaKindLastFrame, urls.LastFrameURL, expiresAt, now)
		if err != nil {
			return nil, err
		}
		records = append(records, media)
	}
	return records, nil
}

func findStringValue(v any, key string) string {
	switch typed := v.(type) {
	case map[string]any:
		if raw, ok := typed[key]; ok {
			if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
			if nested := findStringValue(raw, key); nested != "" {
				return nested
			}
		}
		for _, child := range typed {
			if found := findStringValue(child, key); found != "" {
				return found
			}
		}
	case []any:
		for _, child := range typed {
			if found := findStringValue(child, key); found != "" {
				return found
			}
		}
	}
	return ""
}

func ArchiveAIUnionTaskMediaAsync(ctx context.Context, task *model.Task, urls AIUnionMediaURLs) {
	records, err := EnsureAIUnionMediaRecordsForTask(task, urls)
	if err != nil {
		common.SysLog("failed to create ai union media records: " + err.Error())
		return
	}
	for _, media := range records {
		if media == nil || media.Status == model.AiUnionMediaStatusReady {
			continue
		}
		mediaID := media.ID
		gopool.Go(func() {
			if err := DownloadAIUnionMedia(context.Background(), mediaID); err != nil {
				common.SysLog("failed to archive ai union media: " + err.Error())
			}
		})
	}
}

func RecoverPendingAIUnionMediaArchives(ctx context.Context, limit int) {
	if limit <= 0 {
		limit = 1000
	}
	now := time.Now().Unix()
	cutoff := now - aiUnionSourceURLTTLSeconds
	var media []*model.AiUnionMedia
	if err := model.DB.Where("status IN ?", []string{
		model.AiUnionMediaStatusPending,
		model.AiUnionMediaStatusDownloading,
		model.AiUnionMediaStatusFailed,
	}).Where("source_expires_at = 0 OR source_expires_at > ?", now).
		Order("id asc").
		Limit(limit).
		Find(&media).Error; err != nil {
		common.SysLog("failed to scan ai union media archives: " + err.Error())
		return
	}
	for _, item := range media {
		if item == nil {
			continue
		}
		mediaID := item.ID
		gopool.Go(func() {
			if err := DownloadAIUnionMedia(context.Background(), mediaID); err != nil {
				common.SysLog("failed to recover ai union media archive: " + err.Error())
			}
		})
	}

	var tasks []*model.Task
	if err := model.DB.Where("status = ? AND action = ? AND (finish_time > ? OR (finish_time = 0 AND updated_at > ?))",
		model.TaskStatusSuccess, AIUnionTaskAction, cutoff, cutoff).
		Order("id asc").
		Limit(limit).
		Find(&tasks).Error; err != nil {
		common.SysLog("failed to scan ai union completed tasks: " + err.Error())
		return
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		ArchiveAIUnionTaskMediaAsync(ctx, task, ExtractAIUnionMediaURLsFromTask(task))
	}
}

func ensureAIUnionMediaRecord(task *model.Task, kind string, sourceURL string, expiresAt int64, now int64) (*model.AiUnionMedia, error) {
	var media model.AiUnionMedia
	err := model.DB.Where("user_id = ? AND task_id = ? AND kind = ?", task.UserId, task.TaskID, kind).
		First(&media).Error
	exists, err := model.RecordExist(err)
	if err != nil {
		return nil, err
	}
	if exists {
		if media.Status != model.AiUnionMediaStatusReady && media.SourceURL != sourceURL {
			media.SourceURL = sourceURL
			media.SourceExpiresAt = expiresAt
			media.Status = model.AiUnionMediaStatusPending
			media.Error = ""
			media.UpdatedAt = now
			if err := model.DB.Save(&media).Error; err != nil {
				return nil, err
			}
		}
		return &media, nil
	}

	media = model.AiUnionMedia{
		UserId:          task.UserId,
		TaskID:          task.TaskID,
		Kind:            kind,
		Status:          model.AiUnionMediaStatusPending,
		SourceURL:       sourceURL,
		SourceExpiresAt: expiresAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := model.DB.Create(&media).Error; err != nil {
		return nil, err
	}
	return &media, nil
}
