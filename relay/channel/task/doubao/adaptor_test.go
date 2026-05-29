package doubao

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestFetchTaskUsesCloudwiseSeedancePath(t *testing.T) {
	service.InitHttpClient()

	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	adaptor := &TaskAdaptor{}
	resp, err := adaptor.FetchTask(server.URL, "test-key", map[string]any{"task_id": "upstream-task"}, "")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "/api/v3/contents/generations/tasks/upstream-task", gotPath)

	resp, err = adaptor.FetchTask(server.URL+"/cloudwise.ai", "test-key", map[string]any{"task_id": "upstream-task"}, "")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "/cloudwise.ai/api/v1/aiproducts/video/seedance/tasks/upstream-task", gotPath)

	resp, err = adaptor.FetchTask(server.URL+"/api/v1/cloudwise.ai", "test-key", map[string]any{"task_id": "upstream-task"}, "")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "/api/v1/cloudwise.ai/api/v1/aiproducts/video/seedance/tasks/upstream-task", gotPath)

	resp, err = adaptor.FetchTask(server.URL, "test-key", map[string]any{
		"task_id":           "upstream-task",
		"origin_model_name": "dreamina-seedance-2-0-fast-260128",
	}, "")
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, "/api/v1/aiproducts/video/seedance/tasks/upstream-task", gotPath)
}

func TestSeedanceURLDoesNotDuplicateAPIVersion(t *testing.T) {
	require.Equal(
		t,
		"https://api.cloudwise.ai/api/v1/aiproducts/video/seedance",
		seedanceURL("https://api.cloudwise.ai/api/v1", "/api/v1/aiproducts/video/seedance"),
	)
	require.Equal(
		t,
		"https://www.qreel.ai/api/v1/aiproducts/video/seedance",
		seedanceURL("https://www.qreel.ai", "/api/v1/aiproducts/video/seedance"),
	)
}

func TestParseTaskResultSupportsNestedSeedanceURLs(t *testing.T) {
	body := []byte(`{
		"data": {
			"status": "succeeded",
			"content": {
				"video_url": "https://media.example/video.mp4",
				"last_frame_url": "https://media.example/last.jpg"
			}
		}
	}`)

	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult(body)
	require.NoError(t, err)
	require.Equal(t, model.TaskStatusSuccess, result.Status)
	require.Equal(t, "https://media.example/video.mp4", result.Url)
}

func TestDoResponseAcceptsWrappedSeedanceSubmitID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: io.NopCloser(strings.NewReader(`{
			"code": 10000,
			"message": "success",
			"data": {"id": "upstream-task-id"}
		}`)),
	}
	info := &relaycommon.RelayInfo{
		OriginModelName: "dreamina-seedance-2-0-fast-260128",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
	}

	adaptor := &TaskAdaptor{}
	taskID, _, taskErr := adaptor.DoResponse(c, resp, info)
	require.Nil(t, taskErr)
	require.Equal(t, "upstream-task-id", taskID)
}

func TestDoResponseAcceptsSeedanceSubmitIDVariants(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "data string",
			body: `{"code":10000,"message":"success","data":"upstream-task-data-string"}`,
			want: "upstream-task-data-string",
		},
		{
			name: "camel task id",
			body: `{"code":10000,"message":"success","data":{"taskId":"upstream-task-camel"}}`,
			want: "upstream-task-camel",
		},
		{
			name: "nested result id",
			body: `{"code":10000,"message":"success","result":{"id":"upstream-task-result"}}`,
			want: "upstream-task-result",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(tc.body)),
			}
			info := &relaycommon.RelayInfo{
				OriginModelName: "dreamina-seedance-2-0-fast-260128",
				TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
			}

			adaptor := &TaskAdaptor{}
			taskID, _, taskErr := adaptor.DoResponse(c, resp, info)
			require.Nil(t, taskErr)
			require.Equal(t, tc.want, taskID)
		})
	}
}

func TestConvertToRequestPayloadDropsAIUnionChannelHints(t *testing.T) {
	req := &relaycommon.TaskSubmitReq{
		Prompt:  "让她给我跳一段舞蹈",
		Model:   "dreamina-seedance-2-0-fast-260128",
		Seconds: "4",
		Metadata: map[string]interface{}{
			"_ai_union_channel_id": 1,
			"ratio":                "16:9",
			"resolution":           "480p",
			"generate_audio":       true,
			"watermark":            false,
			"seed":                 -1,
			"content": []interface{}{
				map[string]interface{}{
					"type":       "image_url",
					"image_url":  map[string]interface{}{"url": "asset://asset-test"},
					"role":       "reference_image",
					"channel_id": 1,
				},
			},
		},
	}

	adaptor := &TaskAdaptor{}
	payload, err := adaptor.convertToRequestPayload(req)
	require.NoError(t, err)
	require.Len(t, payload.Content, 2)
	require.Equal(t, "image_url", payload.Content[0].Type)
	require.Equal(t, "asset://asset-test", payload.Content[0].ImageURL.URL)
	require.Equal(t, "reference_image", payload.Content[0].Role)
	require.Equal(t, "text", payload.Content[1].Type)
	require.Equal(t, req.Prompt, payload.Content[1].Text)

	body, err := common.Marshal(payload)
	require.NoError(t, err)
	bodyText := string(body)
	require.NotContains(t, bodyText, "channel_id")
	require.NotContains(t, bodyText, "_ai_union_channel_id")
	require.Contains(t, bodyText, `"duration":4`)
	require.Contains(t, bodyText, `"seed":-1`)
	require.Contains(t, bodyText, `"watermark":false`)
}
