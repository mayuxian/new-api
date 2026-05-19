# Seedance 视频生成 API 接口文档

所有 API 请求需在 Header 中携带 API Key：

```
Authorization: Bearer ak_xxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

---

## 一、视频生成

### 1. 创建视频生成任务

**POST** `/api/v1/aiproducts/video/seedance`

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| model | string | 是 | 模型名称，见下方支持的模型列表 |
| content | array | 是 | 内容数组，支持 `text` / `image_url` / `video_url` / `audio_url` |
| ratio | string | 否 | 宽高比：`16:9`, `9:16`, `4:3`, `3:4`, `1:1`, `21:9`, `adaptive`，默认 `adaptive` |
| resolution | string | 否 | 分辨率：`480p`, `720p`, `1080p`，默认 `720p` |
| duration | integer | 否 | 视频时长（秒）：[4, 15] 范围内的整数，或 `-1`（智能选择），默认 5 |
| generate_audio | boolean | 否 | 是否生成同步音频，默认 true |
| enhance | boolean | 否 | 是否进行视频增强（1080P超分增强，最高码率兼顾质量与成本），默认 false |
| seed | integer | 否 | 随机种子，[-1, 2^32-1]，-1 为随机，默认 -1 |
| return_last_frame | boolean | 否 | 是否返回最后一帧图片（用于连续视频生成），默认 false |
| watermark | boolean | 否 | 是否添加水印，默认 false |

#### content 数组元素说明

| 类型 | 字段 | 说明 |
|------|------|------|
| `text` | `text` | 提示词文本 |
| `image_url` | `image_url.url` | 图片引用，使用 `asset://<assetId>` 格式或公开 URL |
| `video_url` | `video_url.url` | 视频引用，使用 `asset://<assetId>` 格式或公开 URL |
| `audio_url` | `audio_url.url` | 音频引用，使用公开 URL、Base64 编码或 `asset://<assetId>` 格式 |

**`image_url` 和 `video_url` 支持的 role 值：**

| role | 说明 |
|------|------|
| `first_frame` | 视频首帧（仅 image_url） |
| `last_frame` | 视频尾帧（仅 image_url） |
| `reference_image` | 参考图，最多 9 张（仅 image_url） |
| `reference_video` | 参考视频（仅 video_url） |
| `reference_audio` | 参考音频（仅 audio_url） |

> ⚠️ **重要**：参考图/视频的 URL 必须使用 `asset://<assetId>` 格式，即先上传素材到私有库，等待审核通过后使用返回的 asset ID。不要直接使用图片 URL 或签名 URL。

#### 素材输入要求

**图片要求**：
- 格式：jpeg, png, webp, bmp, tiff, gif
- 分辨率：宽高均在 300px ~ 6000px 之间
- 总像素：640×640 ~ 2206×946 范围内
- 大小：单张不超过 20 MB

**视频要求**（仅 Seedance 2.0 系列）：
- 格式：mp4, mov（编码：H.264/AVC, H.265/HEVC；音频：AAC, MP3）
- 分辨率：480p, 720p, 1080p
- 时长：单个视频 [2, 15] 秒，最多 3 个参考视频，总时长不超过 15 秒
- 宽高比：[0.4, 2.5]，宽高像素 [300, 6000]
- 大小：单个不超过 50 MB
- 帧率：[24, 60] FPS

**音频要求**（仅 Seedance 2.0 系列）：
- 格式：wav, mp3
- 时长：单个音频 [2, 15] 秒，最多 3 个参考音频，总时长不超过 15 秒
- 大小：单个不超过 15 MB，请求体总大小不超过 64 MB
- ⚠️ **音频不能单独输入**，必须同时包含至少一个参考视频或图片
- 支持 Base64 编码：格式为 `data:audio/<格式>;base64,<编码内容>`，如 `data:audio/wav;base64,{base64_audio}`

#### 支持的模型

| 模型名称 | 说明 |
|------|------|
| `dreamina-seedance-2-0-260128` | Seedance 2.0 标准版 |
| `dreamina-seedance-2-0-fast-260128` | Seedance 2.0 快速版 |

#### 请求示例

**文本生成视频**：

```bash
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [{"type": "text", "text": "A cute cat walking in a garden"}],
    "ratio": "16:9",
    "duration": 5,
    "watermark": false
  }'
```

**首帧 + 尾帧生成视频**：

```bash
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "镜头从室内平滑推移到室外花园"},
      {"type": "image_url", "image_url": {"url": "asset://asset-xxxx"}, "role": "first_frame"},
      {"type": "image_url", "image_url": {"url": "asset://asset-yyyy"}, "role": "last_frame"}
    ],
    "ratio": "16:9",
    "duration": 5
  }'
```

**参考图生成视频**（最多 9 张参考图）：

```bash
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "角色在草地上奔跑，阳光明媚"},
      {"type": "image_url", "image_url": {"url": "asset://asset-xxxx"}, "role": "reference_image"},
      {"type": "image_url", "image_url": {"url": "asset://asset-yyyy"}, "role": "reference_image"}
    ],
    "ratio": "16:9",
    "duration": 5
  }'
```

**参考视频生成视频**：

```bash
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "同样的舞蹈动作，但在赛博朋克城市场景中"},
      {"type": "video_url", "video_url": {"url": "https://example.com/dance.mp4"}, "role": "reference_video"}
    ],
    "ratio": "16:9",
    "duration": 5
  }'
```

**生成视频 + 自动音频**：

```bash
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [{"type": "text", "text": "雷雨中的森林，闪电划过天空"}],
    "generate_audio": true,
    "ratio": "16:9",
    "duration": 5
  }'
```

**参考音频 + 参考视频生成视频**（Seedance 2.0）：

```bash
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [
      {"type": "text", "text": "人物随着音乐节拍跳舞"},
      {"type": "video_url", "video_url": {"url": "asset://asset-xxxx"}, "role": "reference_video"},
      {"type": "audio_url", "audio_url": {"url": "asset://asset-yyyy"}, "role": "reference_audio"}
    ],
    "ratio": "16:9",
    "duration": 5
  }'
```

**连续视频生成**（使用 return_last_frame）：

```bash
# 第一段视频，返回最后一帧
curl -X POST https://www.qreel.ai/api/v1/aiproducts/video/seedance \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "dreamina-seedance-2-0-260128",
    "content": [{"type": "text", "text": "A hero walks through the forest"}],
    "return_last_frame": true,
    "ratio": "16:9",
    "duration": 5
  }'
# 查询完成后，从响应的 content.last_frame_url 获取最后一帧图片
# 将其作为下一段视频的 first_frame 输入，实现无缝衔接
```

#### 响应示例

```json
{
  "id": "MTQ5MTQ5NjQ4MDQ2MzEyNjUy..."
}
```

---

### 2. 查询视频任务状态

**GET** `/api/v1/aiproducts/video/seedance/tasks/{taskId}`

#### 请求示例

```bash
curl https://www.qreel.ai/api/v1/aiproducts/video/seedance/tasks/MTQ5MTQ5NjQ4MDQ2MzEyNjUy... \
  -H "Authorization: Bearer ak_your_api_key"
```

#### 状态说明

| status | 说明 | 操作 |
|--------|------|------|
| `submitted` | 任务已提交 | 继续轮询 |
| `running` | 视频生成中 | 继续轮询（5\~10秒间隔） |
| `succeeded` | 生成完成 | 从 `content.video_url` 下载 |
| `failed` | 生成失败 | 查看 `error.message` |

#### 成功响应示例

```json
{
  "id": "1491496480463126528",
  "model": "dreamina-seedance-2-0-260128",
  "status": "succeeded",
  "content": {
    "video_url": "https://...xxx.mp4?...",
    "last_frame_url": "https://...last-frame.png?...",   // 仅当 return_last_frame=true 时返回
    "expireAt": 1775728526
  },
  "usage": {
    "completion_tokens": 246840,
    "total_tokens": 246840
  },
  "resolution": "1080p",
  "ratio": "16:9",
  "duration": 5
}
```

---

## 二、私有素材库

### 1. 创建素材分组

**POST** `/api/v1/assets/groups/create`

#### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | 是 | 分组名称（最多64字符） |
| description | string | 否 | 分组描述 |
| groupType | string | 否 | 分组类型，默认 AIGC |
| projectName | string | 否 | 项目名称，默认 default |

#### 请求示例

```bash
curl -X POST https://www.qreel.ai/api/v1/assets/groups/create \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-references", "description": "Reference images"}'
```

#### 响应示例

```json
{
  "code": 10000,
  "message": "success",
  "data": {"id": "group-20260408173943-m2bxd"}
}
```

---

### 2. 上传素材

#### 方式一：通过 URL 上传

**POST** `/api/v1/assets/create`

传入可公开访问的文件 URL，服务端会自动下载并注册到私有素材库。

##### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| groupId | string | 是 | 目标分组 ID |
| url | string | 是 | 可公开访问的图片/视频/音频 URL |
| assetType | string | 否 | `Image`、`Video` 或 `Audio`，默认 Image |
| name | string | 否 | 素材名称 |

> ⚠️ **图片要求**：宽高均需在 300px ~ 6000px 之间，URL 必须可公开访问。

##### 请求示例

```bash
curl -X POST https://www.qreel.ai/api/v1/assets/create \
  -H "Authorization: Bearer ak_your_api_key" \
  -H "Content-Type: application/json" \
  -d '{
    "groupId": "group-20260408173943-m2bxd",
    "url": "https://example.com/my-reference.jpg",
    "assetType": "Image",
    "name": "sunset-ref"
  }'
```

##### 响应示例

```json
{
  "code": 10000,
  "message": "success",
  "data": {"id": "asset-20260408174237-k2729"}
}
```

#### 方式二：本地文件上传

**POST** `/api/v1/assets/upload`

直接上传本地文件，服务端会自动存储并注册到私有素材库，同时保存到「我的素材 → 创作素材」中。

> ℹ️ 请求格式：`multipart/form-data`

##### 请求参数

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| file | file | 是 | 本地文件（图片/视频/音频） |
| groupId | string | 是 | 目标分组 ID |
| assetType | string | 否 | `Image`、`Video` 或 `Audio`，默认 Image |
| name | string | 否 | 素材名称，默认使用文件名 |

##### 文件限制

| 类型 | 支持格式 | 大小限制 |
|------|---------|---------|
| Image | jpg, png, webp, bmp, gif | 10 MB |
| Video | mp4, mov, webm, avi | 100 MB |
| Audio | mp3, wav, aac, m4a, ogg | 20 MB |

##### 请求示例

```bash
curl -X POST https://www.qreel.ai/api/v1/assets/upload \
  -H "Authorization: Bearer ak_your_api_key" \
  -F "file=@/path/to/local-image.jpg" \
  -F "groupId=group-20260408173943-m2bxd" \
  -F "assetType=Image" \
  -F "name=sunset-ref"
```

##### 响应示例

```json
{
  "code": 10000,
  "message": "success",
  "data": {
    "id": "asset-20260420143320-mzrx8",
    "url": "https://tos-bucket.example.com/202604/assets/images/20260420_abc123.jpg"
  }
}
```

> ℹ️ 本地上传的文件会自动保存到「我的素材 → 创作素材」中，方便后续查看和管理。响应中额外返回 `url` 字段，可直接用于其他接口。

---

### 3. 查询素材状态

**POST** `/api/v1/assets/get`

#### 请求参数

```json
{"id": "asset-20260408174237-k2729"}
```

#### 状态说明

| Status | 说明 | 操作 |
|--------|------|------|
| `Processing` | 处理/审核中 | 继续轮询（3\~5秒间隔） |
| `Active` | 可用 | 使用 `asset://<assetId>` 格式引用素材 |
| `Failed` | 处理失败 | 检查图片要求后重试 |

> ℹ️ **示例**：素材 ID 为 `asset-20260408174237-k2729`，则在视频生成时传入 `"url": "asset://asset-20260408174237-k2729"`。

#### 成功响应示例

```json
{
  "code": 10000,
  "message": "success",
  "data": {
    "Id": "asset-20260408174237-k2729",
    "Name": "my-reference-image",
    "URL": "https://ark-media-asset-...",
    "AssetType": "Image",
    "Status": "Active",
    "CreateTime": "2026-04-08T09:42:37Z"
  }
}
```

---

## 三、错误码

### HTTP 状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 成功（业务错误请检查 body 中的 code 字段） |
| 401 | 未授权 — API Key 无效或已停用 |
| 502 | 网关错误 — 上游服务不可用 |

### 常见错误响应

**API Key 无效 (401)**

```json
{"success": false, "message": "Unauthorized", "status": 401}
```

**模型不支持**

```json
{"success": false, "message": "Model xxx is not supported"}
```
