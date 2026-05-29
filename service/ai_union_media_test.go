package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestAIUnionMediaTokenRejectsExpiredAndTamperedTokens(t *testing.T) {
	token, err := GenerateAIUnionMediaToken(12, 34, time.Minute)
	require.NoError(t, err)

	claims, err := VerifyAIUnionMediaToken(token)
	require.NoError(t, err)
	require.Equal(t, int64(12), claims.MediaID)
	require.Equal(t, 34, claims.UserID)

	_, err = VerifyAIUnionMediaToken(token + "x")
	require.Error(t, err)

	expired, err := GenerateAIUnionMediaToken(12, 34, -time.Minute)
	require.NoError(t, err)
	_, err = VerifyAIUnionMediaToken(expired)
	require.Error(t, err)
}

func TestDownloadAIUnionMediaPersistsFileAndUpdatesRecord(t *testing.T) {
	truncate(t)
	mediaDir := t.TempDir()
	t.Setenv("AI_UNION_MEDIA_DIR", mediaDir)

	body := []byte("video-bytes")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	media := &model.AiUnionMedia{
		UserId:          7,
		TaskID:          "task_test",
		Kind:            model.AiUnionMediaKindVideo,
		Status:          model.AiUnionMediaStatusPending,
		SourceURL:       server.URL + "/video.mp4",
		SourceExpiresAt: time.Now().Add(time.Hour).Unix(),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, model.DB.Create(media).Error)

	require.NoError(t, DownloadAIUnionMedia(context.Background(), media.ID))

	var stored model.AiUnionMedia
	require.NoError(t, model.DB.First(&stored, media.ID).Error)
	require.Equal(t, model.AiUnionMediaStatusReady, stored.Status)
	require.Equal(t, "video/mp4", stored.MimeType)
	require.Equal(t, int64(len(body)), stored.SizeBytes)
	require.NotEmpty(t, stored.Sha256)
	require.NotEmpty(t, stored.StoragePath)
	require.False(t, filepath.IsAbs(stored.StoragePath))
	require.Greater(t, stored.DownloadedAt, int64(0))

	fullPath := AIUnionMediaFullPath(stored.StoragePath)
	got, err := os.ReadFile(fullPath)
	require.NoError(t, err)
	require.Equal(t, body, got)
}
