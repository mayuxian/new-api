package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const (
	aiUnionDefaultModel     = "dreamina-seedance-2-0-fast-260128"
	aiUnionMediaTokenTTL    = 10 * time.Minute
	aiUnionAssetTokenTTL    = time.Hour
	aiUnionMaxUploadSize    = 128 << 20
	aiUnionVideoSubmitPath  = "/v1/video/generations"
	aiUnionAssetTaskPrefix  = "asset_"
	aiUnionAssetStoragePath = "assets"
)

var aiUnionModelCapabilityOrder = []string{
	"dreamina-seedance-2-0-260128",
	aiUnionDefaultModel,
}

var (
	aiUnionFastModelResolutions     = []string{"480p", "720p"}
	aiUnionStandardModelResolutions = []string{"480p", "720p", "1080p"}
)

type aiUnionModelConfig struct {
	Model            string   `json:"model"`
	ChannelAvailable bool     `json:"channel_available"`
	PriceAvailable   bool     `json:"price_available"`
	UsePrice         bool     `json:"use_price"`
	RatioOrPrice     float64  `json:"ratio_or_price"`
	Resolutions      []string `json:"resolutions"`
}

type aiUnionDefaultTokenStatus struct {
	Exists    bool   `json:"exists"`
	Enabled   bool   `json:"enabled"`
	MaskedKey string `json:"masked_key"`
	Status    int    `json:"status"`
	Group     string `json:"group"`
}

type aiUnionTaskResponse struct {
	Task  *model.Task           `json:"task"`
	Media []*model.AiUnionMedia `json:"media"`
}

type aiUnionUpstreamAsset struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

type aiUnionUpstreamAssetResponse struct {
	Code    int `json:"code"`
	Data    aiUnionUpstreamAsset
	ID      string `json:"id"`
	URL     string `json:"url"`
	Message string `json:"message"`
}

type aiUnionCaptureWriter struct {
	gin.ResponseWriter
	header http.Header
	body   bytes.Buffer
	status int
}

func newAIUnionCaptureWriter(writer gin.ResponseWriter) *aiUnionCaptureWriter {
	return &aiUnionCaptureWriter{
		ResponseWriter: writer,
		header:         http.Header{},
		status:         http.StatusOK,
	}
}

func (w *aiUnionCaptureWriter) Header() http.Header {
	return w.header
}

func (w *aiUnionCaptureWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *aiUnionCaptureWriter) WriteHeaderNow() {}

func (w *aiUnionCaptureWriter) Write(data []byte) (int, error) {
	return w.body.Write(data)
}

func (w *aiUnionCaptureWriter) WriteString(s string) (int, error) {
	return w.body.WriteString(s)
}

func (w *aiUnionCaptureWriter) Written() bool {
	return w.body.Len() > 0
}

func (w *aiUnionCaptureWriter) Size() int {
	return w.body.Len()
}

func AIUnionConfig(c *gin.Context) {
	token, err := ensureAIUnionDefaultToken(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	models, defaultModel, err := aiUnionModelConfigsForToken(token)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"default_model":           defaultModel,
		"media_token_ttl_seconds": int(aiUnionMediaTokenTTL.Seconds()),
		"default_token": aiUnionDefaultTokenStatus{
			Exists:    token != nil,
			Enabled:   token != nil && token.Status == common.TokenStatusEnabled,
			MaskedKey: token.GetMaskedKey(),
			Status:    token.Status,
			Group:     token.Group,
		},
		"models": models,
	})
}

func AIUnionUploadAsset(c *gin.Context) {
	token, err := ensureAIUnionDefaultToken(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Status != common.TokenStatusEnabled {
		common.ApiErrorMsg(c, "Default API key is disabled")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, aiUnionMaxUploadSize)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer file.Close()

	modelName := strings.TrimSpace(c.PostForm("model"))
	if modelName == "" {
		modelName, err = aiUnionDefaultModelForToken(token)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	allowed, err := aiUnionModelAllowedForToken(token, modelName)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !allowed {
		common.ApiErrorMsg(c, fmt.Sprintf("AI Union model %s is not available for the default API key", modelName))
		return
	}

	if err := prepareAIUnionRelayContext(c, token, modelName, 0); err != nil {
		common.ApiError(c, err)
		return
	}

	media, err := saveAIUnionAsset(c, file, header)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		common.ApiError(c, err)
		return
	}
	upstreamAsset, err := uploadAIUnionAssetToUpstream(c, file, media.FileName, media.MimeType)
	if err != nil {
		media.Status = model.AiUnionMediaStatusFailed
		media.Error = err.Error()
		media.UpdatedAt = time.Now().Unix()
		_ = model.DB.Save(media).Error
		common.ApiError(c, err)
		return
	}
	media.SourceURL = "asset://" + upstreamAsset.ID
	media.Error = ""
	media.UpdatedAt = time.Now().Unix()
	if err := model.DB.Save(media).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	tokenValue, err := service.GenerateAIUnionMediaToken(media.ID, media.UserId, aiUnionAssetTokenTTL)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"asset_id":          media.TaskID,
		"upstream_asset_id": upstreamAsset.ID,
		"upstream_url":      media.SourceURL,
		"channel_id":        common.GetContextKeyInt(c, constant.ContextKeyChannelId),
		"media":             media,
		"expires_at":        time.Now().Add(aiUnionAssetTokenTTL).Unix(),
		"url":               buildAIUnionPublicMediaURL(c, media.ID, tokenValue),
		"token":             tokenValue,
	})
}

func AIUnionAssetStatus(c *gin.Context) {
	assetID := c.Param("asset_id")
	userID := c.GetInt("id")
	var media model.AiUnionMedia
	err := model.DB.Where("user_id = ? AND task_id = ? AND kind = ?", userID, assetID, model.AiUnionMediaKindAsset).
		First(&media).Error
	exists, err := model.RecordExist(err)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists {
		common.ApiErrorMsg(c, "asset not found")
		return
	}
	common.ApiSuccess(c, &media)
}

func AIUnionSubmitTask(c *gin.Context) {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Prompt) == "" {
		common.ApiErrorMsg(c, "prompt is required")
		return
	}

	token, err := ensureAIUnionDefaultToken(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if token.Status != common.TokenStatusEnabled {
		common.ApiErrorMsg(c, "Default API key is disabled. Please enable it on the API keys page.")
		return
	}
	if strings.TrimSpace(req.Model) == "" {
		req.Model, err = aiUnionDefaultModelForToken(token)
		if err != nil {
			common.ApiError(c, err)
			return
		}
	}
	allowed, err := aiUnionModelAllowedForToken(token, req.Model)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !allowed {
		common.ApiErrorMsg(c, fmt.Sprintf("AI Union model %s is not available for the default API key", req.Model))
		return
	}
	if err := normalizeAIUnionRequest(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	preferredChannelID, err := aiUnionPreferredChannelID(req.Metadata)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := prepareAIUnionRelayContext(c, token, req.Model, preferredChannelID); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := replaceRequestBody(c, &req); err != nil {
		common.ApiError(c, err)
		return
	}

	originalPath := c.Request.URL.Path
	originalRequestURI := c.Request.RequestURI
	c.Request.URL.Path = aiUnionVideoSubmitPath
	c.Request.RequestURI = aiUnionVideoSubmitPath
	defer func() {
		c.Request.URL.Path = originalPath
		c.Request.RequestURI = originalRequestURI
	}()

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	realWriter := c.Writer
	capture := newAIUnionCaptureWriter(realWriter)
	c.Writer = capture
	task, taskErr := SubmitTaskWithRelayInfo(c, relayInfo)
	c.Writer = realWriter
	if taskErr != nil {
		common.ApiErrorMsg(c, taskErr.Message)
		return
	}
	if task == nil {
		common.ApiErrorMsg(c, "task creation failed")
		return
	}

	task.Action = service.AIUnionTaskAction
	task.PrivateData.AiUnion = true
	if err := task.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, aiUnionTaskResponse{Task: task, Media: []*model.AiUnionMedia{}})
}

func AIUnionListTasks(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	if pageQuery, err := strconv.Atoi(c.Query("page")); err == nil && pageQuery > 0 {
		pageInfo.Page = pageQuery
	}

	var tasks []*model.Task
	query := model.DB.Model(&model.Task{}).Where("user_id = ? AND action = ?", userID, service.AIUnionTaskAction)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	if err := query.
		Order("id desc").
		Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).
		Find(&tasks).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(tasks)
	common.ApiSuccess(c, pageInfo)
}

func AIUnionGetTask(c *gin.Context) {
	task, ok := getAIUnionTaskForUser(c)
	if !ok {
		return
	}
	media, err := getAndMaybeArchiveAIUnionMedia(c, task)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, aiUnionTaskResponse{Task: task, Media: media})
}

func AIUnionTaskMedia(c *gin.Context) {
	task, ok := getAIUnionTaskForUser(c)
	if !ok {
		return
	}
	media, err := getAndMaybeArchiveAIUnionMedia(c, task)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, media)
}

func AIUnionMediaToken(c *gin.Context) {
	mediaID, err := strconv.ParseInt(c.Param("media_id"), 10, 64)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	media, exists, err := model.GetAiUnionMediaByUser(mediaID, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !exists || media == nil {
		common.ApiErrorMsg(c, "media not found")
		return
	}
	if media.Status != model.AiUnionMediaStatusReady {
		common.ApiErrorMsg(c, "media is not ready")
		return
	}

	token, err := service.GenerateAIUnionMediaToken(media.ID, media.UserId, aiUnionMediaTokenTTL)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"token":      token,
		"expires_at": time.Now().Add(aiUnionMediaTokenTTL).Unix(),
		"url":        buildAIUnionPublicMediaURL(c, media.ID, token),
	})
}

func AIUnionPublicMedia(c *gin.Context) {
	mediaID, err := strconv.ParseInt(c.Param("media_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	claims, err := service.VerifyAIUnionMediaToken(c.Query("token"))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": err.Error()})
		return
	}
	if claims.MediaID != mediaID {
		c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "media token does not match media"})
		return
	}
	media, exists, err := model.GetAiUnionMediaByID(mediaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	if !exists || media == nil || media.UserId != claims.UserID {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "media not found"})
		return
	}
	if media.Status != model.AiUnionMediaStatusReady || media.StoragePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "media is not ready"})
		return
	}
	fullPath := service.AIUnionMediaFullPath(media.StoragePath)
	file, err := os.Open(fullPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "media file not found"})
		return
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	if media.MimeType != "" {
		c.Header("Content-Type", media.MimeType)
	}
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, sanitizeAIUnionFileName(media.FileName)))
	http.ServeContent(c.Writer, c.Request, media.FileName, stat.ModTime(), file)
}

func ensureAIUnionDefaultToken(c *gin.Context) (*model.Token, error) {
	userID := c.GetInt("id")
	username := c.GetString("username")
	userCache, err := model.GetUserCache(userID)
	if err != nil {
		return nil, err
	}
	userCache.WriteContext(c)
	return model.EnsureUserDefaultToken(userID, username)
}

func prepareAIUnionRelayContext(c *gin.Context, token *model.Token, modelName string, preferredChannelID int) error {
	userCache, err := model.GetUserCache(token.UserId)
	if err != nil {
		return err
	}
	userCache.WriteContext(c)

	usingGroup := userCache.Group
	if token.Group != "" {
		if _, ok := service.GetUserUsableGroups(userCache.Group)[token.Group]; !ok {
			return fmt.Errorf("no permission to use group %s", token.Group)
		}
		if !ratio_setting.ContainsGroupRatio(token.Group) && token.Group != "auto" {
			return fmt.Errorf("group %s is deprecated", token.Group)
		}
		usingGroup = token.Group
	}
	common.SetContextKey(c, constant.ContextKeyUsingGroup, usingGroup)
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())
	if err := middleware.SetupContextForToken(c, token); err != nil {
		return err
	}
	return setupAIUnionChannelContext(c, usingGroup, modelName, preferredChannelID)
}

func setupAIUnionChannelContext(c *gin.Context, usingGroup string, modelName string, preferredChannelID int) error {
	if preferredChannelID > 0 {
		return setupAIUnionPreferredChannelContext(c, usingGroup, modelName, preferredChannelID)
	}

	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(&service.RetryParam{
		Ctx:        c,
		TokenGroup: usingGroup,
		ModelName:  modelName,
		Retry:      common.GetPointer(0),
	})
	if err != nil {
		return fmt.Errorf("get AI Union channel failed: %w", err)
	}
	if channel == nil {
		return fmt.Errorf("no available channel for AI Union model %s in group %s", modelName, usingGroup)
	}
	if usingGroup == "auto" && selectGroup != "" {
		common.SetContextKey(c, constant.ContextKeyAutoGroup, selectGroup)
	}
	if newAPIError := middleware.SetupContextForSelectedChannel(c, channel, modelName); newAPIError != nil {
		return newAPIError.Err
	}
	return nil
}

func setupAIUnionPreferredChannelContext(c *gin.Context, usingGroup string, modelName string, channelID int) error {
	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		return fmt.Errorf("get AI Union asset channel failed: %w", err)
	}
	if channel.Status != common.ChannelStatusEnabled {
		return fmt.Errorf("AI Union asset channel %d is disabled", channelID)
	}

	if usingGroup == "auto" {
		userGroup := common.GetContextKeyString(c, constant.ContextKeyUserGroup)
		for _, group := range service.GetUserAutoGroup(userGroup) {
			if model.IsChannelEnabledForGroupModel(group, modelName, channelID) {
				common.SetContextKey(c, constant.ContextKeyAutoGroup, group)
				if newAPIError := middleware.SetupContextForSelectedChannel(c, channel, modelName); newAPIError != nil {
					return newAPIError.Err
				}
				return nil
			}
		}
		return fmt.Errorf("AI Union asset channel %d is not available for model %s", channelID, modelName)
	}

	if !model.IsChannelEnabledForGroupModel(usingGroup, modelName, channelID) {
		return fmt.Errorf("AI Union asset channel %d is not available for group %s and model %s", channelID, usingGroup, modelName)
	}
	if newAPIError := middleware.SetupContextForSelectedChannel(c, channel, modelName); newAPIError != nil {
		return newAPIError.Err
	}
	return nil
}

func replaceRequestBody(c *gin.Context, req *relaycommon.TaskSubmitReq) error {
	body, err := common.Marshal(req)
	if err != nil {
		return err
	}
	storage, err := common.CreateBodyStorage(body)
	if err != nil {
		return err
	}
	c.Set(common.KeyBodyStorage, storage)
	if _, err := storage.Seek(0, io.SeekStart); err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(storage)
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return nil
}

func getAIUnionTaskForUser(c *gin.Context) (*model.Task, bool) {
	taskID := c.Param("task_id")
	var task model.Task
	err := model.DB.Where("user_id = ? AND task_id = ? AND action = ?", c.GetInt("id"), taskID, service.AIUnionTaskAction).
		First(&task).Error
	exists, err := model.RecordExist(err)
	if err != nil {
		common.ApiError(c, err)
		return nil, false
	}
	if !exists {
		common.ApiErrorMsg(c, "task not found")
		return nil, false
	}
	return &task, true
}

func getAndMaybeArchiveAIUnionMedia(c *gin.Context, task *model.Task) ([]*model.AiUnionMedia, error) {
	if task.Status == model.TaskStatusSuccess {
		service.ArchiveAIUnionTaskMediaAsync(context.Background(), task, service.ExtractAIUnionMediaURLsFromTask(task))
	}
	return model.GetAiUnionMediaByTask(c.GetInt("id"), task.TaskID)
}

func normalizeAIUnionRequest(req *relaycommon.TaskSubmitReq) error {
	if !aiUnionModelSupported(req.Model) {
		return fmt.Errorf("unsupported AI Union model: %s", req.Model)
	}

	if strings.TrimSpace(req.Seconds) == "" {
		req.Seconds = "5"
	}
	seconds, err := strconv.Atoi(req.Seconds)
	if err != nil || seconds < 4 || seconds > 15 {
		return fmt.Errorf("duration must be an integer from 4 to 15 seconds")
	}
	req.Seconds = strconv.Itoa(seconds)
	req.Duration = seconds

	if req.Metadata == nil {
		req.Metadata = map[string]interface{}{}
	}
	resolution := strings.ToLower(strings.TrimSpace(aiUnionMetadataString(req.Metadata["resolution"])))
	if resolution == "" {
		resolution = "720p"
	}
	if !aiUnionResolutionAllowed(req.Model, resolution) {
		return fmt.Errorf("resolution %s is not supported by %s", resolution, req.Model)
	}
	req.Metadata["resolution"] = resolution

	if _, ok := req.Metadata["seed"]; !ok {
		req.Metadata["seed"] = -1
	}

	return nil
}

func aiUnionModelSupported(modelName string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(modelName)), "seedance")
}

func aiUnionModelResolutions(modelName string) []string {
	if !aiUnionModelSupported(modelName) {
		return []string{}
	}
	if strings.Contains(strings.ToLower(modelName), "fast") {
		return append([]string(nil), aiUnionFastModelResolutions...)
	}
	return append([]string(nil), aiUnionStandardModelResolutions...)
}

func aiUnionResolutionAllowed(modelName string, resolution string) bool {
	for _, candidate := range aiUnionModelResolutions(modelName) {
		if candidate == resolution {
			return true
		}
	}
	return false
}

func aiUnionModelConfigsForToken(token *model.Token) ([]aiUnionModelConfig, string, error) {
	modelNames, userGroup, usingGroup, err := aiUnionModelNamesForToken(token)
	if err != nil {
		return nil, "", err
	}
	models := make([]aiUnionModelConfig, 0, len(modelNames))
	for _, modelName := range modelNames {
		ratioOrPrice, usePrice, priceAvailable := ratio_setting.GetModelRatioOrPrice(modelName)
		models = append(models, aiUnionModelConfig{
			Model:            modelName,
			ChannelAvailable: aiUnionChannelAvailable(userGroup, usingGroup, modelName),
			PriceAvailable:   priceAvailable,
			UsePrice:         usePrice,
			RatioOrPrice:     ratioOrPrice,
			Resolutions:      aiUnionModelResolutions(modelName),
		})
	}
	return models, aiUnionDefaultModelFromConfigs(models), nil
}

func aiUnionModelNamesForToken(token *model.Token) ([]string, string, string, error) {
	if token == nil {
		return nil, "", "", fmt.Errorf("token is nil")
	}
	userCache, err := model.GetUserCache(token.UserId)
	if err != nil {
		return nil, "", "", err
	}
	userGroup := userCache.Group
	usingGroup, err := aiUnionUsingGroupForToken(userGroup, token)
	if err != nil {
		return nil, "", "", err
	}

	available := make(map[string]bool)
	if token.ModelLimitsEnabled {
		for _, modelName := range token.GetModelLimits() {
			modelName = strings.TrimSpace(modelName)
			if aiUnionModelSupported(modelName) {
				available[modelName] = true
			}
		}
	} else if usingGroup == "auto" {
		for _, group := range service.GetUserAutoGroup(userGroup) {
			for _, modelName := range model.GetGroupEnabledModels(group) {
				if aiUnionModelSupported(modelName) {
					available[modelName] = true
				}
			}
		}
	} else {
		for _, modelName := range model.GetGroupEnabledModels(usingGroup) {
			if aiUnionModelSupported(modelName) {
				available[modelName] = true
			}
		}
	}

	modelNames := make([]string, 0, len(available))
	for _, modelName := range aiUnionModelCapabilityOrder {
		if available[modelName] {
			modelNames = append(modelNames, modelName)
			delete(available, modelName)
		}
	}
	if len(available) > 0 {
		remaining := make([]string, 0, len(available))
		for modelName := range available {
			remaining = append(remaining, modelName)
		}
		sort.Strings(remaining)
		modelNames = append(modelNames, remaining...)
	}
	return modelNames, userGroup, usingGroup, nil
}

func aiUnionUsingGroupForToken(userGroup string, token *model.Token) (string, error) {
	if token == nil || strings.TrimSpace(token.Group) == "" {
		return userGroup, nil
	}
	tokenGroup := strings.TrimSpace(token.Group)
	if _, ok := service.GetUserUsableGroups(userGroup)[tokenGroup]; !ok {
		return "", fmt.Errorf("no permission to use group %s", tokenGroup)
	}
	if !ratio_setting.ContainsGroupRatio(tokenGroup) && tokenGroup != "auto" {
		return "", fmt.Errorf("group %s is deprecated", tokenGroup)
	}
	return tokenGroup, nil
}

func aiUnionDefaultModelFromConfigs(models []aiUnionModelConfig) string {
	for _, item := range models {
		if item.Model == aiUnionDefaultModel && item.ChannelAvailable && item.PriceAvailable {
			return item.Model
		}
	}
	for _, item := range models {
		if item.ChannelAvailable && item.PriceAvailable {
			return item.Model
		}
	}
	for _, item := range models {
		if item.Model == aiUnionDefaultModel {
			return item.Model
		}
	}
	if len(models) > 0 {
		return models[0].Model
	}
	return ""
}

func aiUnionDefaultModelForToken(token *model.Token) (string, error) {
	models, defaultModel, err := aiUnionModelConfigsForToken(token)
	if err != nil {
		return "", err
	}
	if defaultModel == "" || len(models) == 0 {
		return "", fmt.Errorf("no AI Union video model is available for the default API key")
	}
	return defaultModel, nil
}

func aiUnionModelAllowedForToken(token *model.Token, modelName string) (bool, error) {
	models, _, err := aiUnionModelConfigsForToken(token)
	if err != nil {
		return false, err
	}
	for _, item := range models {
		if item.Model == modelName {
			return true, nil
		}
	}
	return false, nil
}

func aiUnionMetadataString(value interface{}) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", value)
	}
}

func aiUnionPreferredChannelID(metadata map[string]interface{}) (int, error) {
	if metadata == nil {
		return 0, nil
	}
	if id := aiUnionMetadataInt(metadata["_ai_union_channel_id"]); id > 0 {
		return id, nil
	}
	rawContent, ok := metadata["content"]
	if !ok {
		return 0, nil
	}
	content, ok := rawContent.([]interface{})
	if !ok {
		return 0, nil
	}

	channelID := 0
	for _, rawItem := range content {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			continue
		}
		id := aiUnionMetadataInt(item["channel_id"])
		if id <= 0 {
			continue
		}
		if channelID == 0 {
			channelID = id
			continue
		}
		if channelID != id {
			return 0, fmt.Errorf("referenced assets must use the same upstream channel")
		}
	}
	return channelID, nil
}

func aiUnionMetadataInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		id := int(v)
		if float64(id) == v {
			return id
		}
	case string:
		id, _ := strconv.Atoi(strings.TrimSpace(v))
		return id
	}
	return 0
}

func aiUnionChannelAvailable(userGroup string, usingGroup string, modelName string) bool {
	if usingGroup == "auto" {
		for _, group := range service.GetUserAutoGroup(userGroup) {
			ch, err := model.GetRandomSatisfiedChannel(group, modelName, 0)
			if err == nil && ch != nil {
				return true
			}
		}
		return false
	}
	ch, err := model.GetRandomSatisfiedChannel(usingGroup, modelName, 0)
	return err == nil && ch != nil
}

func saveAIUnionAsset(c *gin.Context, file multipart.File, header *multipart.FileHeader) (*model.AiUnionMedia, error) {
	userID := c.GetInt("id")
	random, err := common.GenerateRandomCharsKey(24)
	if err != nil {
		return nil, err
	}
	assetID := aiUnionAssetTaskPrefix + random
	now := time.Now().Unix()
	media := &model.AiUnionMedia{
		UserId:    userID,
		TaskID:    assetID,
		Kind:      model.AiUnionMediaKindAsset,
		Status:    model.AiUnionMediaStatusDownloading,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := model.DB.Create(media).Error; err != nil {
		return nil, err
	}

	mimeType, err := detectUploadedAIUnionMime(file, header)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" || len(ext) > 12 {
		ext = ".bin"
	}
	relativePath := filepath.Join(strconv.Itoa(userID), aiUnionAssetStoragePath, fmt.Sprintf("%s-%d%s", assetID, media.ID, ext))
	fullPath := service.AIUnionMediaFullPath(relativePath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return nil, err
	}
	dst, err := os.Create(fullPath)
	if err != nil {
		return nil, err
	}
	hasher := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(dst, hasher), file)
	closeErr := dst.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}

	media.Status = model.AiUnionMediaStatusReady
	media.StoragePath = relativePath
	media.FileName = filepath.Base(header.Filename)
	media.MimeType = mimeType
	media.SizeBytes = size
	media.Sha256 = hex.EncodeToString(hasher.Sum(nil))
	media.DownloadedAt = time.Now().Unix()
	media.UpdatedAt = media.DownloadedAt
	if err := model.DB.Save(media).Error; err != nil {
		return nil, err
	}
	return media, nil
}

func uploadAIUnionAssetToUpstream(c *gin.Context, file multipart.File, fileName string, mimeType string) (*aiUnionUpstreamAsset, error) {
	baseURL := common.GetContextKeyString(c, constant.ContextKeyChannelBaseUrl)
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	if baseURL == "" || apiKey == "" {
		return nil, fmt.Errorf("AI Union upstream channel is not ready")
	}

	proxy := ""
	if channelSetting, ok := common.GetContextKeyType[dto.ChannelSettings](c, constant.ContextKeyChannelSetting); ok {
		proxy = channelSetting.Proxy
	}

	groupID, err := createAIUnionUpstreamAssetGroup(baseURL, apiKey, proxy)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}
	if err := writer.WriteField("groupId", groupID); err != nil {
		return nil, err
	}
	if err := writer.WriteField("assetType", aiUnionAssetTypeFromMime(mimeType)); err != nil {
		return nil, err
	}
	if err := writer.WriteField("name", buildAIUnionUpstreamAssetName(fileName)); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, aiUnionSeedanceURL(baseURL, "/api/v1/assets/upload"), &requestBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := doAIUnionUpstreamRequest(req, proxy)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	asset, err := parseAIUnionUpstreamAssetResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("upload AI Union asset failed: %w", err)
	}
	return asset, nil
}

func createAIUnionUpstreamAssetGroup(baseURL string, apiKey string, proxy string) (string, error) {
	payload, err := common.Marshal(gin.H{
		"name":        "c6c-ai-union",
		"description": "C6C AI Union uploads",
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, aiUnionSeedanceURL(baseURL, "/api/v1/assets/groups/create"), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := doAIUnionUpstreamRequest(req, proxy)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	asset, err := parseAIUnionUpstreamAssetResponse(resp)
	if err != nil {
		return "", fmt.Errorf("create AI Union asset group failed: %w", err)
	}
	if asset.ID == "" {
		return "", fmt.Errorf("create AI Union asset group failed: empty group id")
	}
	return asset.ID, nil
}

func doAIUnionUpstreamRequest(req *http.Request, proxy string) (*http.Response, error) {
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func parseAIUnionUpstreamAssetResponse(resp *http.Response) (*aiUnionUpstreamAsset, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed aiUnionUpstreamAssetResponse
	if err := common.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	asset := parsed.Data
	if asset.ID == "" {
		asset.ID = parsed.ID
	}
	if asset.URL == "" {
		asset.URL = parsed.URL
	}
	if asset.ID == "" {
		if parsed.Message != "" {
			return nil, fmt.Errorf("%s", parsed.Message)
		}
		return nil, fmt.Errorf("empty upstream asset id")
	}
	return &asset, nil
}

func aiUnionSeedanceURL(baseURL string, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if strings.HasSuffix(baseURL, "/api/v1") && strings.HasPrefix(path, "/api/v1/") {
		return baseURL + strings.TrimPrefix(path, "/api/v1")
	}
	return baseURL + path
}

func aiUnionAssetTypeFromMime(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.HasPrefix(mimeType, "video/"):
		return "Video"
	case strings.HasPrefix(mimeType, "audio/"):
		return "Audio"
	default:
		return "Image"
	}
}

func buildAIUnionUpstreamAssetName(fileName string) string {
	fileName = sanitizeAIUnionFileName(fileName)
	if len(fileName) <= 64 {
		return fileName
	}
	ext := filepath.Ext(fileName)
	if len(ext) >= 64 {
		return fileName[:64]
	}
	stem := strings.TrimSuffix(fileName, ext)
	maxStemLength := 64 - len(ext)
	if maxStemLength <= 0 {
		return fileName[:64]
	}
	return stem[:min(len(stem), maxStemLength)] + ext
}

func detectUploadedAIUnionMime(file multipart.File, header *multipart.FileHeader) (string, error) {
	if value := strings.TrimSpace(strings.Split(header.Header.Get("Content-Type"), ";")[0]); value != "" {
		return value, nil
	}
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	return http.DetectContentType(buf[:n]), nil
}

func buildAIUnionPublicMediaURL(c *gin.Context, mediaID int64, token string) string {
	base := strings.TrimRight(system_setting.ServerAddress, "/")
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil || c.Request.Header.Get("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		base = scheme + "://" + c.Request.Host
	}
	return fmt.Sprintf("%s/api/ai-union/public/media/%d?token=%s", base, mediaID, token)
}

func sanitizeAIUnionFileName(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "media"
	}
	return strings.ReplaceAll(name, `"`, "")
}
