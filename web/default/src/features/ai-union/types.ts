/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data?: T
}

export type AiUnionDefaultToken = {
  exists: boolean
  enabled: boolean
  masked_key: string
  status: number
  group: string
}

export type AiUnionModelConfig = {
  model: string
  channel_available: boolean
  price_available: boolean
  use_price: boolean
  ratio_or_price: number
  resolutions: string[]
}

export type AiUnionConfig = {
  default_model: string
  media_token_ttl_seconds: number
  default_token: AiUnionDefaultToken
  models: AiUnionModelConfig[]
}

export type AiUnionMedia = {
  id: number
  user_id: number
  task_id: string
  kind: 'video' | 'last_frame' | 'asset'
  status: 'pending' | 'downloading' | 'ready' | 'failed'
  file_name: string
  mime_type: string
  size_bytes: number
  sha256: string
  source_expires_at: number
  downloaded_at: number
  error: string
  created_at: number
  updated_at: number
}

export type AiUnionTask = {
  id: number
  created_at: number
  updated_at: number
  task_id: string
  user_id: number
  action: string
  status: string
  fail_reason: string
  progress: string
  data?: unknown
  properties?: {
    input?: string
    upstream_model_name?: string
    origin_model_name?: string
  }
}

export type AiUnionTaskDetail = {
  task: AiUnionTask
  media: AiUnionMedia[]
}

export type AiUnionTaskPage = {
  page: number
  page_size: number
  total: number
  items: AiUnionTask[]
}

export type AiUnionTaskListParams = {
  page?: number
  page_size?: number
}

export type AiUnionAssetUpload = {
  asset_id: string
  upstream_asset_id?: string
  upstream_url?: string
  channel_id?: number
  media: AiUnionMedia
  expires_at: number
  url: string
  token: string
}

export type AiUnionMediaToken = {
  token: string
  expires_at: number
  url: string
}

export type AiUnionSubmitPayload = {
  prompt: string
  model: string
  seconds: string
  metadata: Record<string, unknown>
}

export type UploadedAsset = {
  asset_id: string
  upstream_asset_id?: string
  upstream_url?: string
  channel_id?: number
  file_name: string
  mime_type: string
  url: string
  label: string
  type: 'image' | 'video' | 'audio'
}
