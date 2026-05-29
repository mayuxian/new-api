package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
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
	require.NoError(t, db.AutoMigrate(&model.Task{}, &model.User{}, &model.Token{}, &model.Ability{}))

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
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
