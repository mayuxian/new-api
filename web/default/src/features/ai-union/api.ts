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

import { api } from '@/lib/api'
import type {
  AiUnionAssetGroup,
  AiUnionAssetGroupCreatePayload,
  AiUnionAssetUpload,
  AiUnionConfig,
  AiUnionMedia,
  AiUnionMediaToken,
  AiUnionSubmitPayload,
  AiUnionTaskDetail,
  AiUnionTaskListParams,
  AiUnionTaskPage,
  ApiResponse,
} from './types'

export async function getAiUnionConfig() {
  const res = await api.get<ApiResponse<AiUnionConfig>>('/api/ai-union/config')
  return res.data
}

export async function uploadAiUnionAsset(
  file: File,
  model: string,
  options: { groupId?: string; channelId?: number } = {}
) {
  const form = new FormData()
  form.append('file', file)
  form.append('model', model)
  if (options.groupId) form.append('group_id', options.groupId)
  if (options.channelId) form.append('channel_id', String(options.channelId))
  const res = await api.post<ApiResponse<AiUnionAssetUpload>>(
    '/api/ai-union/assets/upload',
    form,
    {
      headers: { 'Content-Type': 'multipart/form-data' },
    }
  )
  return res.data
}

export async function createAiUnionAssetGroup(
  payload: AiUnionAssetGroupCreatePayload
) {
  const res = await api.post<ApiResponse<AiUnionAssetGroup>>(
    '/api/ai-union/assets/groups/create',
    payload
  )
  return res.data
}

export async function submitAiUnionTask(payload: AiUnionSubmitPayload) {
  const res = await api.post<ApiResponse<AiUnionTaskDetail>>(
    '/api/ai-union/tasks',
    payload
  )
  return res.data
}

export async function listAiUnionTasks(params: AiUnionTaskListParams = {}) {
  const res = await api.get<ApiResponse<AiUnionTaskPage>>('/api/ai-union/tasks', {
    params,
  })
  return res.data
}

export async function getAiUnionTask(taskId: string) {
  const res = await api.get<ApiResponse<AiUnionTaskDetail>>(
    `/api/ai-union/tasks/${encodeURIComponent(taskId)}`
  )
  return res.data
}

export async function getAiUnionTaskMedia(taskId: string) {
  const res = await api.get<ApiResponse<AiUnionMedia[]>>(
    `/api/ai-union/tasks/${encodeURIComponent(taskId)}/media`
  )
  return res.data
}

export async function getAiUnionMediaToken(mediaId: number) {
  const res = await api.get<ApiResponse<AiUnionMediaToken>>(
    `/api/ai-union/media/${mediaId}/token`
  )
  return res.data
}
