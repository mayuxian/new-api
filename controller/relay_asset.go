package controller

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// RelayAsset proxies Seedance asset APIs
func RelayAsset(c *gin.Context) {
	channelId, ok := c.Get(string(constant.ContextKeyChannelId))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "channel not found in context",
				"type":    "new_api_error",
			},
		})
		return
	}
	
	id, ok := channelId.(int)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "channel id type error",
				"type":    "new_api_error",
			},
		})
		return
	}

	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "channel not found",
				"type":    "new_api_error",
			},
		})
		return
	}

	baseURL := channel.GetBaseURL()
	requestURL := c.Request.URL.Path
	
	// Determine upstream path
	// The client requests /v1/assets/upload or /v1/assets/create or /v1/assets/groups/create
	upstreamPath := requestURL
	
	// // QReel's official API requires /api/v1/assets/...
	// if strings.Contains(baseURL, "qreel.ai") {
	// 	upstreamPath = strings.Replace(requestURL, "/v1/assets/", "/api/v1/assets/", 1)
	// }
	
	// Ensure we handle edge cases where baseUrl ends with slash and upstream path starts with slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	fullURL := fmt.Sprintf("%s%s", baseURL, upstreamPath)

	req, err := http.NewRequest(c.Request.Method, fullURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "failed to create request",
				"type":    "new_api_error",
			},
		})
		return
	}
	req.ContentLength = c.Request.ContentLength

	// Copy headers
	for k, v := range c.Request.Header {
		req.Header[k] = v
	}
	// Override auth
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	req.Header.Del("Host")

	// Do request
	client, _ := service.GetHttpClientWithProxy("")
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "new_api_error",
			},
		})
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for k, v := range resp.Header {
		c.Writer.Header()[k] = v
	}
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}
