package controller

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAIUnionControllerTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	initModelListColumnNames(t)

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.Token{}, &model.Channel{}, &model.Ability{}, &model.AiUnionMedia{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestAIUnionCreateAssetGroupProxiesToSelectedChannel(t *testing.T) {
	db := setupAIUnionControllerTestDB(t)

	var upstreamAuth string
	var upstreamPayload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/assets/groups/create", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		upstreamAuth = r.Header.Get("Authorization")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, common.Unmarshal(body, &upstreamPayload))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"group_test_123"}}`))
	}))
	defer upstream.Close()

	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "wuji",
		Password: "12345678",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		UserId:      1,
		Key:         "default-token",
		Purpose:     model.TokenPurposeDefault,
		Status:      common.TokenStatusEnabled,
		ExpiredTime: -1,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:      7,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "upstream-key",
		Name:    "ai-union-upstream",
		Status:  common.ChannelStatusEnabled,
		BaseURL: common.GetPointer(upstream.URL),
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     aiUnionDefaultModel,
		ChannelId: 7,
		Enabled:   true,
		Priority:  common.GetPointer(int64(0)),
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/ai-union/assets/groups/create",
		strings.NewReader(`{"model":"`+aiUnionDefaultModel+`","name":"project refs","description":"Reusable refs"}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("username", "wuji")

	AIUnionCreateAssetGroup(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "Bearer upstream-key", upstreamAuth)
	require.Equal(t, "project refs", upstreamPayload.Name)
	require.Equal(t, "Reusable refs", upstreamPayload.Description)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			GroupID   string `json:"group_id"`
			ChannelID int    `json:"channel_id"`
			Model     string `json:"model"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "group_test_123", response.Data.GroupID)
	require.Equal(t, 7, response.Data.ChannelID)
	require.Equal(t, aiUnionDefaultModel, response.Data.Model)
}

func TestAIUnionModelNamesForTokenUsesGroupAbilities(t *testing.T) {
	db := setupAIUnionControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "wuji",
		Password: "12345678",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     aiUnionDefaultModel,
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "dreamina-seedance-2-0-fast-260128-b",
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "default",
		Model:     "not-a-video-model",
		ChannelId: 1,
		Enabled:   true,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group:     "vip",
		Model:     "dreamina-seedance-2-0-260128",
		ChannelId: 1,
		Enabled:   true,
	}).Error)

	modelNames, userGroup, usingGroup, err := aiUnionModelNamesForToken(&model.Token{UserId: 1})

	require.NoError(t, err)
	require.Equal(t, "default", userGroup)
	require.Equal(t, "default", usingGroup)
	require.Equal(t, []string{aiUnionDefaultModel, "dreamina-seedance-2-0-fast-260128-b"}, modelNames)
}

func TestAIUnionModelNamesForTokenUsesTokenModelLimits(t *testing.T) {
	db := setupAIUnionControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "wuji",
		Password: "12345678",
		Group:    "default",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}).Error)

	modelNames, _, _, err := aiUnionModelNamesForToken(&model.Token{
		UserId:             1,
		ModelLimitsEnabled: true,
		ModelLimits:        aiUnionDefaultModel + ",not-a-video-model,dreamina-seedance-2-0-260128,dreamina-seedance-2-0-fast-260128-b",
	})

	require.NoError(t, err)
	require.Equal(t, []string{"dreamina-seedance-2-0-260128", aiUnionDefaultModel, "dreamina-seedance-2-0-fast-260128-b"}, modelNames)
}

func TestAIUnionModelResolutionsFollowFastNaming(t *testing.T) {
	require.Equal(t, []string{"480p", "720p"}, aiUnionModelResolutions("dreamina-seedance-2-0-fast-260128-b"))
	require.Equal(t, []string{"480p", "720p", "1080p"}, aiUnionModelResolutions("dreamina-seedance-2-0-260128-custom"))
}

func TestAIUnionListTasksReturnsPagedResult(t *testing.T) {
	db := setupAIUnionControllerTestDB(t)

	for idx := 1; idx <= 5; idx++ {
		require.NoError(t, db.Create(&model.Task{
			TaskID: fmt.Sprintf("task_user_7_%d", idx),
			UserId: 7,
			Action: service.AIUnionTaskAction,
			Status: model.TaskStatusSuccess,
		}).Error)
	}
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task_other_user",
		UserId: 8,
		Action: service.AIUnionTaskAction,
		Status: model.TaskStatusSuccess,
	}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: "task_other_action",
		UserId: 7,
		Action: "other_action",
		Status: model.TaskStatusSuccess,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/ai-union/tasks?page=2&page_size=2", nil)
	ctx.Set("id", 7)

	AIUnionListTasks(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Page     int          `json:"page"`
			PageSize int          `json:"page_size"`
			Total    int64        `json:"total"`
			Items    []model.Task `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, 2, response.Data.Page)
	require.Equal(t, 2, response.Data.PageSize)
	require.Equal(t, int64(5), response.Data.Total)
	require.Len(t, response.Data.Items, 2)
	require.Equal(t, "task_user_7_3", response.Data.Items[0].TaskID)
	require.Equal(t, "task_user_7_2", response.Data.Items[1].TaskID)
}

func TestAIUnionGetTaskDoesNotExposeUpstreamVideoURL(t *testing.T) {
	db := setupAIUnionControllerTestDB(t)

	task := model.Task{
		TaskID: "task_hidden_video_url",
		UserId: 7,
		Action: service.AIUnionTaskAction,
		Status: model.TaskStatusFailure,
	}
	task.SetData(map[string]any{
		"content": map[string]any{
			"video_url": "https://download.cloudwise.ai/private.mp4?X-Tos-Signature=secret",
		},
	})
	require.NoError(t, db.Create(&task).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/ai-union/tasks/task_hidden_video_url", nil)
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_hidden_video_url"}}
	ctx.Set("id", 7)

	AIUnionGetTask(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "download.cloudwise.ai")
	require.NotContains(t, recorder.Body.String(), "X-Tos-Signature")

	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	data := response["data"].(map[string]any)
	taskPayload := data["task"].(map[string]any)
	require.NotContains(t, taskPayload, "data")
}

func TestAIUnionDeleteTaskRemovesHistoryMediaAndFiles(t *testing.T) {
	db := setupAIUnionControllerTestDB(t)
	mediaDir := t.TempDir()
	t.Setenv("AI_UNION_MEDIA_DIR", mediaDir)

	require.NoError(t, db.Create(&model.Task{
		TaskID: "task_delete",
		UserId: 7,
		Action: service.AIUnionTaskAction,
		Status: model.TaskStatusSuccess,
	}).Error)
	storagePath := filepath.Join("7", "task_delete", "video-1.mp4")
	fullPath := service.AIUnionMediaFullPath(storagePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
	require.NoError(t, os.WriteFile(fullPath, []byte("video"), 0644))
	require.NoError(t, db.Create(&model.AiUnionMedia{
		UserId:      7,
		TaskID:      "task_delete",
		Kind:        model.AiUnionMediaKindVideo,
		Status:      model.AiUnionMediaStatusReady,
		StoragePath: storagePath,
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodDelete, "/api/ai-union/tasks/task_delete", nil)
	ctx.Params = gin.Params{{Key: "task_id", Value: "task_delete"}}
	ctx.Set("id", 7)

	AIUnionDeleteTask(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)

	var taskCount int64
	require.NoError(t, db.Model(&model.Task{}).Where("task_id = ?", "task_delete").Count(&taskCount).Error)
	require.Zero(t, taskCount)

	var mediaCount int64
	require.NoError(t, db.Model(&model.AiUnionMedia{}).Where("task_id = ?", "task_delete").Count(&mediaCount).Error)
	require.Zero(t, mediaCount)
	require.NoFileExists(t, fullPath)
}

func TestBuildAIUnionPublicMediaURLUsesRelativePath(t *testing.T) {
	previousServerAddress := system_setting.ServerAddress
	system_setting.ServerAddress = "http://localhost:3000"
	t.Cleanup(func() {
		system_setting.ServerAddress = previousServerAddress
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:3002/api/ai-union/media/5/token", nil)

	url := buildAIUnionPublicMediaURL(ctx, 5, "token-value")

	require.Equal(t, "/api/ai-union/public/media/5?token=token-value", url)
}
