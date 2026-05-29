package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const aiUnionMediaTokenVersion = "v1"

type AIUnionMediaTokenClaims struct {
	MediaID int64
	UserID  int
	Expires int64
}

func AIUnionMediaDir() string {
	if dir := strings.TrimSpace(os.Getenv("AI_UNION_MEDIA_DIR")); dir != "" {
		return dir
	}
	return filepath.Join(".", "data", "ai-union-media")
}

func AIUnionMediaFullPath(storagePath string) string {
	if filepath.IsAbs(storagePath) {
		return storagePath
	}
	return filepath.Join(AIUnionMediaDir(), filepath.Clean(storagePath))
}

func GenerateAIUnionMediaToken(mediaID int64, userID int, ttl time.Duration) (string, error) {
	if mediaID == 0 || userID == 0 {
		return "", errors.New("media id and user id are required")
	}
	expires := time.Now().Add(ttl).Unix()
	payload := fmt.Sprintf("%s|%d|%d|%d", aiUnionMediaTokenVersion, mediaID, userID, expires)
	signature := common.GenerateHMAC(payload)
	raw := fmt.Sprintf("%s.%d.%d.%d.%s", aiUnionMediaTokenVersion, mediaID, userID, expires, signature)
	return base64.RawURLEncoding.EncodeToString([]byte(raw)), nil
}

func VerifyAIUnionMediaToken(token string) (*AIUnionMediaTokenClaims, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(raw), ".")
	if len(parts) != 5 || parts[0] != aiUnionMediaTokenVersion {
		return nil, errors.New("invalid media token")
	}
	mediaID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return nil, err
	}
	userID, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, err
	}
	expires, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return nil, err
	}
	if expires < time.Now().Unix() {
		return nil, errors.New("media token expired")
	}
	payload := fmt.Sprintf("%s|%d|%d|%d", parts[0], mediaID, userID, expires)
	expected := common.GenerateHMAC(payload)
	if !hmac.Equal([]byte(expected), []byte(parts[4])) {
		return nil, errors.New("invalid media token signature")
	}
	return &AIUnionMediaTokenClaims{
		MediaID: mediaID,
		UserID:  userID,
		Expires: expires,
	}, nil
}

func DeleteAIUnionTaskHistory(userID int, taskID string) (int, error) {
	taskID = strings.TrimSpace(taskID)
	if userID == 0 || taskID == "" {
		return 0, errors.New("user id and task id are required")
	}

	var task model.Task
	err := model.DB.Where("user_id = ? AND task_id = ? AND action = ?", userID, taskID, AIUnionTaskAction).First(&task).Error
	exists, err := model.RecordExist(err)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, errors.New("task not found")
	}
	if task.Status != model.TaskStatusSuccess && task.Status != model.TaskStatusFailure {
		return 0, errors.New("task is not completed")
	}

	var media []*model.AiUnionMedia
	if err := model.DB.Where("user_id = ? AND task_id = ?", userID, taskID).Find(&media).Error; err != nil {
		return 0, err
	}
	storagePaths := make([]string, 0, len(media))
	for _, item := range media {
		if item != nil && strings.TrimSpace(item.StoragePath) != "" {
			storagePaths = append(storagePaths, item.StoragePath)
		}
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ? AND task_id = ?", userID, taskID).Delete(&model.AiUnionMedia{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND user_id = ? AND task_id = ? AND action = ?", task.ID, userID, taskID, AIUnionTaskAction).Delete(&model.Task{}).Error
	}); err != nil {
		return 0, err
	}

	cleanupAIUnionMediaFiles(storagePaths)
	return len(media), nil
}

func DownloadAIUnionMedia(ctx context.Context, mediaID int64) error {
	media, exists, err := model.GetAiUnionMediaByID(mediaID)
	if err != nil {
		return err
	}
	if !exists || media == nil {
		return errors.New("media not found")
	}
	if media.Status == model.AiUnionMediaStatusReady && media.StoragePath != "" {
		return nil
	}
	if media.SourceExpiresAt > 0 && media.SourceExpiresAt < time.Now().Unix() {
		return markAIUnionMediaFailed(media, "source url expired")
	}
	if strings.TrimSpace(media.SourceURL) == "" {
		return markAIUnionMediaFailed(media, "source url is empty")
	}

	if err := setAIUnionMediaStatus(media, model.AiUnionMediaStatusDownloading, ""); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, media.SourceURL, nil)
	if err != nil {
		_ = markAIUnionMediaFailed(media, err.Error())
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		_ = markAIUnionMediaFailed(media, err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err := fmt.Errorf("source returned status %d", resp.StatusCode)
		_ = markAIUnionMediaFailed(media, err.Error())
		return err
	}

	mimeType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	relativePath := buildAIUnionMediaStoragePath(media, mimeType)
	fullPath := AIUnionMediaFullPath(relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		_ = markAIUnionMediaFailed(media, err.Error())
		return err
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(fullPath), ".download-*")
	if err != nil {
		_ = markAIUnionMediaFailed(media, err.Error())
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(tmpFile, hasher), resp.Body)
	closeErr := tmpFile.Close()
	if copyErr != nil {
		_ = markAIUnionMediaFailed(media, copyErr.Error())
		return copyErr
	}
	if closeErr != nil {
		_ = markAIUnionMediaFailed(media, closeErr.Error())
		return closeErr
	}
	if err := os.Rename(tmpPath, fullPath); err != nil {
		_ = markAIUnionMediaFailed(media, err.Error())
		return err
	}

	media.Status = model.AiUnionMediaStatusReady
	media.StoragePath = relativePath
	media.FileName = filepath.Base(relativePath)
	media.MimeType = mimeType
	media.SizeBytes = size
	media.Sha256 = hex.EncodeToString(hasher.Sum(nil))
	media.DownloadedAt = time.Now().Unix()
	media.Error = ""
	media.UpdatedAt = time.Now().Unix()
	return model.DB.Save(media).Error
}

func cleanupAIUnionMediaFiles(storagePaths []string) {
	removedDirs := make(map[string]struct{})
	for _, storagePath := range storagePaths {
		fullPath := AIUnionMediaFullPath(storagePath)
		if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			common.SysLog("failed to remove ai union media file: " + err.Error())
			continue
		}
		dir := filepath.Dir(fullPath)
		if dir != "." && dir != string(filepath.Separator) {
			removedDirs[dir] = struct{}{}
		}
	}
	for dir := range removedDirs {
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) && !errors.Is(err, syscall.ENOTEMPTY) {
			common.SysLog("failed to remove ai union media directory: " + err.Error())
		}
	}
}

func setAIUnionMediaStatus(media *model.AiUnionMedia, status string, message string) error {
	media.Status = status
	media.Error = message
	media.UpdatedAt = time.Now().Unix()
	return model.DB.Save(media).Error
}

func markAIUnionMediaFailed(media *model.AiUnionMedia, message string) error {
	return setAIUnionMediaStatus(media, model.AiUnionMediaStatusFailed, message)
}

func buildAIUnionMediaStoragePath(media *model.AiUnionMedia, mimeType string) string {
	ext := extensionForAIUnionMedia(media, mimeType)
	name := fmt.Sprintf("%s-%d%s", media.Kind, media.ID, ext)
	return filepath.Join(strconv.Itoa(media.UserId), media.TaskID, name)
}

func extensionForAIUnionMedia(media *model.AiUnionMedia, mimeType string) string {
	if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
		return exts[0]
	}
	if parsed, err := url.Parse(media.SourceURL); err == nil {
		ext := strings.ToLower(filepath.Ext(parsed.Path))
		if ext != "" && len(ext) <= 8 {
			return ext
		}
	}
	if media.Kind == model.AiUnionMediaKindLastFrame {
		return ".jpg"
	}
	if media.Kind == model.AiUnionMediaKindVideo {
		return ".mp4"
	}
	return ".bin"
}
