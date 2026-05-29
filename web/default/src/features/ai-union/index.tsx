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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  AlertTriangle,
  ChevronLeft,
  ChevronRight,
  Download,
  ExternalLink,
  FileVideo,
  Image as ImageIcon,
  KeyRound,
  Loader2,
  Music,
  Plus,
  RefreshCw,
  Trash2,
  WandSparkles,
  X,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import {
  deleteAiUnionTask,
  getAiUnionConfig,
  getAiUnionMediaToken,
  getAiUnionTask,
  listAiUnionTasks,
  submitAiUnionTask,
  uploadAiUnionAsset,
} from './api'
import type {
  AiUnionMedia,
  AiUnionModelConfig,
  AiUnionTask,
  AiUnionTaskPage,
  ApiResponse,
  UploadedAsset,
} from './types'

const ratios = ['16:9', '9:16', '1:1', '4:3', '3:4', '21:9']
const fallbackResolutions = ['480p', '720p', '1080p']
const durations = Array.from({ length: 12 }, (_, index) => String(index + 4))

type MentionRange = {
  from: number
  to: number
  query: string
}

function statusVariant(status: string) {
  switch (status) {
    case 'SUCCESS':
    case 'ready':
      return 'default'
    case 'FAILURE':
    case 'failed':
      return 'destructive'
    case 'pending':
    case 'downloading':
    case 'IN_PROGRESS':
    case 'QUEUED':
    case 'SUBMITTED':
      return 'secondary'
    default:
      return 'outline'
  }
}

function formatBytes(bytes: number) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), 3)
  return `${(bytes / 1024 ** index).toFixed(index === 0 ? 0 : 1)} ${units[index]}`
}

function mediaLabel(media: AiUnionMedia, t: (key: string) => string) {
  if (media.kind === 'video') return t('Video')
  if (media.kind === 'last_frame') return t('Last frame')
  return t('Asset')
}

function assetTypeFromMime(mimeType: string): UploadedAsset['type'] {
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  return 'image'
}

function assetIcon(type: UploadedAsset['type']) {
  if (type === 'video') return <FileVideo className='size-3' />
  if (type === 'audio') return <Music className='size-3' />
  return <ImageIcon className='size-3' />
}

function assetContent(asset: UploadedAsset) {
  const url = asset.upstream_asset_id
    ? `asset://${asset.upstream_asset_id}`
    : asset.upstream_url || asset.url
  if (asset.type === 'video') {
    return {
      type: 'video_url',
      video_url: { url },
      role: 'reference_video',
    }
  }
  if (asset.type === 'audio') {
    return {
      type: 'audio_url',
      audio_url: { url },
      role: 'reference_audio',
    }
  }
  return {
    type: 'image_url',
    image_url: { url },
    role: 'reference_image',
  }
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function downloadUrl(url: string, fileName: string) {
  const link = document.createElement('a')
  link.href = url
  link.download = fileName
  link.target = '_blank'
  link.rel = 'noreferrer'
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}

function TokenState({
  enabled,
  maskedKey,
}: {
  enabled: boolean
  maskedKey: string
}) {
  const { t } = useTranslation()
  return (
    <div className='border-border bg-muted/30 flex min-w-0 items-center gap-2 rounded-lg border px-2.5 py-1.5 text-sm'>
      <KeyRound className='text-muted-foreground size-4 shrink-0' />
      <span className='text-muted-foreground shrink-0'>{t('Default key')}</span>
      <span className='min-w-0 truncate font-mono text-xs'>
        {maskedKey || t('Creating')}
      </span>
      <Badge variant={enabled ? 'default' : 'destructive'}>
        {enabled ? t('Enabled') : t('Disabled')}
      </Badge>
    </div>
  )
}

function WorkspacePanel({
  models,
  defaultModel,
  disabled,
}: {
  models: AiUnionModelConfig[]
  defaultModel: string
  disabled: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const promptRef = useRef<HTMLTextAreaElement>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)
  const pendingUploadRangeRef = useRef<MentionRange | null>(null)
  const [prompt, setPrompt] = useState('')
  const [model, setModel] = useState(defaultModel)
  const [ratio, setRatio] = useState('16:9')
  const [duration, setDuration] = useState('5')
  const [resolution, setResolution] = useState('720p')
  const [generateAudio, setGenerateAudio] = useState(true)
  const [watermark, setWatermark] = useState(false)
  const [seed, setSeed] = useState('-1')
  const [assets, setAssets] = useState<UploadedAsset[]>([])
  const [uploading, setUploading] = useState(false)
  const [showAssetMenu, setShowAssetMenu] = useState(false)
  const [mentionRange, setMentionRange] = useState<MentionRange | null>(null)

  useEffect(() => {
    if (defaultModel) setModel(defaultModel)
  }, [defaultModel])

  const selectedModel = useMemo(
    () => models.find((item) => item.model === model),
    [model, models]
  )
  const resolutionOptions = selectedModel?.resolutions?.length
    ? selectedModel.resolutions
    : fallbackResolutions
  const modelUnavailable =
    !selectedModel ||
    !selectedModel.channel_available ||
    !selectedModel.price_available
  const filteredAssets = useMemo(() => {
    const query = mentionRange?.query.toLowerCase() ?? ''
    return assets.filter(
      (asset) =>
        !query ||
        asset.label.toLowerCase().includes(query) ||
        asset.file_name.toLowerCase().includes(query)
    )
  }, [assets, mentionRange])

  useEffect(() => {
    if (!resolutionOptions.includes(resolution)) {
      setResolution(
        resolutionOptions.includes('720p') ? '720p' : resolutionOptions[0]
      )
    }
  }, [resolution, resolutionOptions])

  const submitMutation = useMutation({
    mutationFn: submitAiUnionTask,
    onSuccess: async (res) => {
      if (!res.success) return
      setPrompt('')
      setSeed('-1')
      await queryClient.invalidateQueries({ queryKey: ['ai-union', 'tasks'] })
      toast.success(t('Task submitted'))
    },
  })

  const findMentionRange = (
    value: string,
    cursor: number
  ): MentionRange | null => {
    const prefix = value.slice(0, cursor)
    const match = /(^|\s)@([^\s@]*)$/.exec(prefix)
    if (!match) return null
    const from = match.index + match[1].length
    return { from, to: cursor, query: match[2] }
  }

  const syncMentionMenu = (value: string, cursor: number) => {
    const range = findMentionRange(value, cursor)
    setMentionRange(range)
    setShowAssetMenu(Boolean(range))
  }

  const generateAssetLabel = (type: UploadedAsset['type']) => {
    const prefix =
      type === 'video' ? t('Video') : type === 'audio' ? t('Audio') : t('Image')
    const used = new Set(
      assets
        .filter((asset) => asset.type === type)
        .map((asset) => {
          const match = asset.label.match(/(\d+)$/)
          return match ? Number(match[1]) : null
        })
        .filter((value): value is number => Number.isInteger(value))
    )
    let index = 1
    while (used.has(index)) index += 1
    return `${prefix}${index}`
  }

  const insertAssetMention = (
    asset: UploadedAsset,
    range: MentionRange | null = mentionRange
  ) => {
    const textarea = promptRef.current
    const from = range?.from ?? textarea?.selectionStart ?? prompt.length
    const to = range?.to ?? textarea?.selectionEnd ?? prompt.length
    const mention = `@${asset.label} `
    const next = `${prompt.slice(0, from)}${mention}${prompt.slice(to)}`
    setPrompt(next)
    setShowAssetMenu(false)
    setMentionRange(null)
    window.requestAnimationFrame(() => {
      textarea?.focus()
      const cursor = from + mention.length
      textarea?.setSelectionRange(cursor, cursor)
    })
  }

  const uploadAssetFile = async (file: File, range: MentionRange | null) => {
    setUploading(true)
    try {
      const res = await uploadAiUnionAsset(file, model)
      if (!res.success || !res.data) {
        toast.error(res.message || t('Upload failed'))
        return
      }
      const data = res.data
      const type = assetTypeFromMime(data.media.mime_type || file.type)
      const asset: UploadedAsset = {
        asset_id: data.asset_id,
        upstream_asset_id: data.upstream_asset_id,
        upstream_url: data.upstream_url,
        channel_id: data.channel_id,
        file_name: data.media.file_name || file.name,
        mime_type: data.media.mime_type || file.type,
        url: data.url,
        label: generateAssetLabel(type),
        type,
      }
      setAssets((current) => [...current, asset])
      insertAssetMention(asset, range)
      toast.success(t('Asset uploaded'))
    } finally {
      setUploading(false)
    }
  }

  const handleFileChange = async (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    const files = Array.from(event.currentTarget.files ?? [])
    event.currentTarget.value = ''
    if (files.length === 0) return
    const firstRange = pendingUploadRangeRef.current
    pendingUploadRangeRef.current = null
    for (const [index, file] of files.entries()) {
      await uploadAssetFile(file, index === 0 ? firstRange : null)
    }
  }

  const triggerUpload = (range: MentionRange | null = null) => {
    pendingUploadRangeRef.current = range
    setShowAssetMenu(false)
    fileInputRef.current?.click()
  }

  const removeAsset = (asset: UploadedAsset) => {
    setAssets((current) =>
      current.filter((item) => item.asset_id !== asset.asset_id)
    )
    setPrompt((current) =>
      current
        .replace(new RegExp(`\\s?@${escapeRegExp(asset.label)}\\b`, 'g'), '')
        .replace(/\s{2,}/g, ' ')
        .trimStart()
    )
  }

  const handleSubmit = () => {
    const referencedAssets = assets.filter((asset) =>
      prompt.includes(`@${asset.label}`)
    )
    if (referencedAssets.some((asset) => !asset.upstream_asset_id)) {
      toast.error(t('Please re-upload referenced assets before submitting'))
      return
    }
    const content = referencedAssets.map(assetContent)
    const channelIds = new Set(
      referencedAssets
        .map((asset) => asset.channel_id)
        .filter((id): id is number => typeof id === 'number' && id > 0)
    )
    if (channelIds.size > 1) {
      toast.error(t('Referenced assets must use the same upstream channel'))
      return
    }
    const [assetChannelId] = Array.from(channelIds)
    const metadata: Record<string, unknown> = {
      ratio,
      resolution,
      generate_audio: generateAudio,
      watermark,
      content,
    }
    if (assetChannelId) {
      metadata._ai_union_channel_id = assetChannelId
    }
    const seedValue = Number(seed)
    if (Number.isFinite(seedValue) && seed.trim() !== '') {
      metadata.seed = seedValue
    }

    submitMutation.mutate({
      prompt: prompt.trim(),
      model,
      seconds: duration,
      metadata,
    })
  }

  const submitDisabled =
    disabled ||
    modelUnavailable ||
    uploading ||
    submitMutation.isPending ||
    prompt.trim().length === 0

  return (
    <Card className='min-h-[560px] rounded-lg'>
      <CardHeader>
        <CardTitle className='flex items-center gap-2'>
          <WandSparkles className='size-4' />
          {t('Workspace')}
        </CardTitle>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='space-y-2'>
          <Label htmlFor='ai-union-prompt'>{t('Prompt')}</Label>
          <div className='relative'>
            <Textarea
              ref={promptRef}
              id='ai-union-prompt'
              value={prompt}
              onChange={(event) => {
                setPrompt(event.target.value)
                syncMentionMenu(event.target.value, event.target.selectionStart)
              }}
              onClick={(event) =>
                syncMentionMenu(
                  event.currentTarget.value,
                  event.currentTarget.selectionStart
                )
              }
              onKeyUp={(event) =>
                syncMentionMenu(
                  event.currentTarget.value,
                  event.currentTarget.selectionStart
                )
              }
              onBlur={() => {
                window.setTimeout(() => setShowAssetMenu(false), 120)
              }}
              className='min-h-32 resize-y'
              placeholder={t('Type @ to attach uploaded assets')}
            />
            {showAssetMenu && (
              <div className='bg-popover text-popover-foreground absolute top-11 left-3 z-20 w-72 overflow-hidden rounded-lg border shadow-md'>
                <div className='text-muted-foreground border-b px-3 py-2 text-xs'>
                  {t('Uploaded assets')}
                </div>
                <div className='max-h-52 overflow-y-auto py-1'>
                  {filteredAssets.length === 0 ? (
                    <div className='text-muted-foreground px-3 py-3 text-sm'>
                      {t('No uploaded assets')}
                    </div>
                  ) : (
                    filteredAssets.map((asset) => (
                      <button
                        key={asset.asset_id}
                        type='button'
                        className='hover:bg-accent flex w-full items-center gap-2 px-3 py-2 text-left text-sm'
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => insertAssetMention(asset)}
                      >
                        {assetIcon(asset.type)}
                        <span className='min-w-0 flex-1 truncate'>
                          {asset.label}
                        </span>
                        <span className='text-muted-foreground max-w-28 truncate text-xs'>
                          {asset.file_name}
                        </span>
                      </button>
                    ))
                  )}
                </div>
                <button
                  type='button'
                  className='hover:bg-accent flex w-full items-center gap-2 border-t px-3 py-2 text-sm'
                  onMouseDown={(event) => event.preventDefault()}
                  onClick={() => triggerUpload(mentionRange)}
                >
                  <Plus className='size-3' />
                  {t('Upload new asset')}
                </button>
              </div>
            )}
          </div>
        </div>

        <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-3'>
          <div className='space-y-2'>
            <Label>{t('Model')}</Label>
            <Select
              items={models.map((item) => ({
                value: item.model,
                label: item.model,
              }))}
              value={model}
              onValueChange={(value) => value && setModel(value)}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {models.map((item) => (
                    <SelectItem key={item.model} value={item.model}>
                      <span className='truncate'>{item.model}</span>
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Ratio')}</Label>
            <Select
              items={ratios.map((item) => ({ value: item, label: item }))}
              value={ratio}
              onValueChange={(value) => value && setRatio(value)}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {ratios.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Duration')}</Label>
            <Select
              items={durations.map((item) => ({
                value: item,
                label: `${item}s`,
              }))}
              value={duration}
              onValueChange={(value) => value && setDuration(value)}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {durations.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}s
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Resolution')}</Label>
            <Select
              items={resolutionOptions.map((item) => ({
                value: item,
                label: item,
              }))}
              value={resolution}
              onValueChange={(value) => value && setResolution(value)}
            >
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {resolutionOptions.map((item) => (
                    <SelectItem key={item} value={item}>
                      {item}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='ai-union-seed'>{t('Seed')}</Label>
            <Input
              id='ai-union-seed'
              value={seed}
              inputMode='numeric'
              onChange={(event) => setSeed(event.target.value)}
            />
          </div>
        </div>

        <div className='grid gap-3 sm:grid-cols-2'>
          <label className='border-border flex items-center justify-between gap-3 rounded-lg border px-3 py-2'>
            <span className='text-sm'>{t('Audio')}</span>
            <Switch
              checked={generateAudio}
              onCheckedChange={setGenerateAudio}
            />
          </label>
          <label className='border-border flex items-center justify-between gap-3 rounded-lg border px-3 py-2'>
            <span className='text-sm'>{t('Watermark')}</span>
            <Switch checked={watermark} onCheckedChange={setWatermark} />
          </label>
        </div>

        <div className='space-y-2'>
          <Label htmlFor='ai-union-asset'>{t('Assets')}</Label>
          <div className='flex flex-wrap items-center gap-2'>
            <Input
              ref={fileInputRef}
              id='ai-union-asset'
              type='file'
              accept='image/*,video/*,audio/*'
              className='hidden'
              onChange={handleFileChange}
              disabled={uploading}
            />
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={uploading}
              onClick={() => triggerUpload()}
            >
              <Plus className='size-4' />
              {t('Upload new asset')}
            </Button>
            {uploading && (
              <Badge variant='secondary'>
                <Loader2 className='size-3 animate-spin' />
                {t('Uploading')}
              </Badge>
            )}
          </div>
          {assets.length > 0 && (
            <div className='flex flex-wrap gap-2'>
              {assets.map((asset) => (
                <Badge key={asset.asset_id} variant='outline'>
                  {assetIcon(asset.type)}
                  <span className='font-medium'>{asset.label}</span>
                  <span className='text-muted-foreground max-w-44 truncate'>
                    {asset.file_name}
                  </span>
                  <button
                    type='button'
                    className='hover:text-destructive ml-1'
                    aria-label={t('Remove asset')}
                    onClick={() => removeAsset(asset)}
                  >
                    <X className='size-3' />
                  </button>
                </Badge>
              ))}
            </div>
          )}
        </div>

        {modelUnavailable && (
          <div className='border-destructive/30 bg-destructive/5 text-destructive flex items-center gap-2 rounded-lg border px-3 py-2 text-sm'>
            <AlertTriangle className='size-4 shrink-0' />
            <span>{t('Channel or pricing is not configured')}</span>
          </div>
        )}

        <Button
          type='button'
          className='w-full sm:w-auto'
          disabled={submitDisabled}
          onClick={handleSubmit}
        >
          {submitMutation.isPending ? (
            <Loader2 className='size-4 animate-spin' />
          ) : (
            <WandSparkles className='size-4' />
          )}
          {t('Submit task')}
        </Button>
      </CardContent>
    </Card>
  )
}

function MediaActions({ media }: { media: AiUnionMedia }) {
  const { t } = useTranslation()
  const [url, setUrl] = useState('')
  const [loading, setLoading] = useState(false)

  const ensureUrl = async () => {
    if (url) return url
    setLoading(true)
    try {
      const res = await getAiUnionMediaToken(media.id)
      if (res.success && res.data) {
        setUrl(res.data.url)
        return res.data.url
      }
    } finally {
      setLoading(false)
    }
    return ''
  }

  useEffect(() => {
    if (media.status === 'ready' && media.kind === 'video') {
      void ensureUrl()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [media.id, media.kind, media.status])

  if (media.status !== 'ready') {
    return (
      <div className='text-muted-foreground text-xs'>
        {media.status === 'failed'
          ? media.error || t('Archive failed')
          : t('Archiving')}
      </div>
    )
  }

  return (
    <div className='space-y-2'>
      {media.kind === 'video' && url && (
        <video
          src={url}
          controls
          className='bg-muted aspect-video w-full rounded-lg object-contain'
        />
      )}
      {media.kind === 'last_frame' && url && (
        <img
          src={url}
          alt={t('Last frame')}
          className='bg-muted aspect-video w-full rounded-lg object-contain'
        />
      )}
      <div className='flex flex-wrap gap-2'>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={loading}
          onClick={async () => {
            const nextUrl = await ensureUrl()
            if (nextUrl) window.open(nextUrl, '_blank', 'noreferrer')
          }}
        >
          {loading ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <ExternalLink className='size-3.5' />
          )}
          {t('Open')}
        </Button>
        <Button
          type='button'
          size='sm'
          variant='outline'
          disabled={loading}
          onClick={async () => {
            const nextUrl = await ensureUrl()
            if (nextUrl) downloadUrl(nextUrl, media.file_name)
          }}
        >
          <Download className='size-3.5' />
          {t('Download')}
        </Button>
      </div>
    </div>
  )
}

function TaskItem({ task }: { task: AiUnionTask }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [isDeleted, setIsDeleted] = useState(false)
  const shouldPoll = !['SUCCESS', 'FAILURE'].includes(task.status)
  const detailQuery = useQuery({
    queryKey: ['ai-union', 'task', task.task_id],
    queryFn: () => getAiUnionTask(task.task_id),
    enabled: !isDeleted,
    refetchInterval: (query) => {
      const detail = query.state.data?.data
      if (!detail) return shouldPoll ? 5000 : false
      const mediaPending = detail.media.some((item) =>
        ['pending', 'downloading'].includes(item.status)
      )
      return detail.task.status !== 'FAILURE' &&
        (detail.task.status !== 'SUCCESS' || mediaPending)
        ? 5000
        : false
    },
  })
  const detail = detailQuery.data?.data
  const currentTask = detail?.task ?? task
  const media = detail?.media ?? []
  const canDelete = ['SUCCESS', 'FAILURE'].includes(currentTask.status)
  const deleteMutation = useMutation({
    mutationFn: () => deleteAiUnionTask(task.task_id),
    onMutate: async () => {
      setDeleteOpen(false)
      setIsDeleted(true)
      await queryClient.cancelQueries({
        queryKey: ['ai-union', 'task', task.task_id],
      })
      queryClient.removeQueries({
        queryKey: ['ai-union', 'task', task.task_id],
      })
      queryClient.setQueriesData<ApiResponse<AiUnionTaskPage>>(
        { queryKey: ['ai-union', 'tasks'] },
        (old) => {
          if (!old?.data?.items) return old
          const items = old.data.items.filter(
            (item) => item.task_id !== task.task_id
          )
          if (items.length === old.data.items.length) return old
          return {
            ...old,
            data: {
              ...old.data,
              total: Math.max(0, old.data.total - 1),
              items,
            },
          }
        }
      )
    },
    onSuccess: async (res) => {
      if (!res.success) {
        setIsDeleted(false)
        toast.error(res.message || t('Failed to delete history'))
        await queryClient.invalidateQueries({ queryKey: ['ai-union', 'tasks'] })
        return
      }
      toast.success(t('History deleted'))
      await queryClient.invalidateQueries({ queryKey: ['ai-union', 'tasks'] })
    },
    onError: async (error) => {
      setIsDeleted(false)
      toast.error((error as Error)?.message || t('Failed to delete history'))
      await queryClient.invalidateQueries({ queryKey: ['ai-union', 'tasks'] })
    },
  })

  if (isDeleted) return null

  return (
    <>
      <div className='border-border bg-card grid gap-4 rounded-lg border p-4 text-sm lg:grid-cols-[minmax(0,1fr)_minmax(280px,420px)]'>
        <div className='min-w-0 space-y-3'>
          <div className='flex flex-wrap items-center gap-2'>
            <Badge variant={statusVariant(currentTask.status)}>
              {currentTask.status}
            </Badge>
            <span className='text-muted-foreground font-mono text-xs'>
              {currentTask.task_id}
            </span>
          </div>
          <div className='text-muted-foreground flex flex-wrap gap-x-4 gap-y-1 text-xs'>
            <span>{currentTask.progress || '0%'}</span>
            <span>
              {dayjs.unix(currentTask.created_at).format('YYYY-MM-DD HH:mm')}
            </span>
            <span>{currentTask.properties?.origin_model_name}</span>
          </div>
          {currentTask.fail_reason && (
            <div className='text-destructive text-sm'>
              {currentTask.fail_reason}
            </div>
          )}
          <div className='flex flex-wrap gap-2'>
            <Button
              type='button'
              size='sm'
              variant='outline'
              onClick={() => void detailQuery.refetch()}
            >
              <RefreshCw
                className={cn(
                  'size-3.5',
                  detailQuery.isFetching && 'animate-spin'
                )}
              />
              {t('Refresh')}
            </Button>
            <Button
              type='button'
              size='sm'
              variant='outline'
              disabled={!canDelete || deleteMutation.isPending}
              onClick={() => setDeleteOpen(true)}
              className='text-destructive hover:text-destructive'
            >
              {deleteMutation.isPending ? (
                <Loader2 className='size-3.5 animate-spin' />
              ) : (
                <Trash2 className='size-3.5' />
              )}
              {t('Delete')}
            </Button>
          </div>
        </div>

        <div className='space-y-3'>
          {media.length === 0 ? (
            <div className='text-muted-foreground rounded-lg border border-dashed px-3 py-6 text-center text-sm'>
              {currentTask.status === 'SUCCESS'
                ? t('Archiving')
                : t('No media yet')}
            </div>
          ) : (
            media.map((item) => (
              <div key={item.id} className='space-y-2 rounded-lg border p-3'>
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <div className='flex min-w-0 items-center gap-2'>
                    {item.kind === 'video' ? (
                      <FileVideo className='text-muted-foreground size-4 shrink-0' />
                    ) : (
                      <ImageIcon className='text-muted-foreground size-4 shrink-0' />
                    )}
                    <span className='truncate font-medium'>
                      {mediaLabel(item, t)}
                    </span>
                  </div>
                  <Badge variant={statusVariant(item.status)}>
                    {item.status}
                  </Badge>
                </div>
                <div className='text-muted-foreground text-xs'>
                  {formatBytes(item.size_bytes)}
                </div>
                <MediaActions media={item} />
              </div>
            ))
          )}
        </div>
      </div>
      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title={t('Confirm delete')}
        desc={t(
          'This will permanently delete this history record and archived media files. Continue?'
        )}
        destructive
        isLoading={deleteMutation.isPending}
        confirmText={t('Delete')}
        handleConfirm={() => deleteMutation.mutate()}
      />
    </>
  )
}

function HistoryPanel() {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const [pageSize, setPageSize] = useState(10)
  const tasksQuery = useQuery({
    queryKey: ['ai-union', 'tasks', page, pageSize],
    queryFn: () => listAiUnionTasks({ page, page_size: pageSize }),
    refetchInterval: 10000,
    placeholderData: (previousData) => previousData,
  })
  const taskPage = tasksQuery.data?.data
  const tasks = taskPage?.items ?? []
  const total = taskPage?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  useEffect(() => {
    if (page > totalPages) {
      setPage(totalPages)
    }
  }, [page, totalPages])

  return (
    <Card className='rounded-lg'>
      <CardHeader>
        <CardTitle className='flex items-center justify-between gap-3'>
          <span>{t('History')}</span>
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void tasksQuery.refetch()}
          >
            <RefreshCw
              className={cn(
                'size-3.5',
                tasksQuery.isFetching && 'animate-spin'
              )}
            />
            {t('Refresh')}
          </Button>
        </CardTitle>
      </CardHeader>
      <CardContent>
        {tasks.length === 0 ? (
          <div className='text-muted-foreground rounded-lg border border-dashed px-3 py-10 text-center text-sm'>
            {t('No tasks yet')}
          </div>
        ) : (
          <div className='space-y-3'>
            {tasks.map((task) => (
              <TaskItem key={task.task_id} task={task} />
            ))}
          </div>
        )}
        {total > 0 && (
          <div className='border-border mt-4 flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
            <div className='text-muted-foreground text-xs'>
              {t('Total')}: {total}
            </div>
            <div className='flex flex-wrap items-center gap-2'>
              <div className='text-muted-foreground flex items-center gap-2 text-xs'>
                <span>{t('Rows per page')}</span>
                <Select
                  value={String(pageSize)}
                  onValueChange={(value) => {
                    setPageSize(Number(value))
                    setPage(1)
                  }}
                >
                  <SelectTrigger className='h-8 w-[76px]'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='10'>10</SelectItem>
                    <SelectItem value='20'>20</SelectItem>
                    <SelectItem value='50'>50</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className='text-muted-foreground min-w-[96px] text-center text-xs'>
                {t('Page {{current}} of {{total}}', {
                  current: total === 0 ? 0 : page,
                  total: totalPages,
                })}
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page <= 1 || tasksQuery.isFetching}
                onClick={() => setPage((current) => Math.max(1, current - 1))}
              >
                <ChevronLeft className='size-3.5' />
                {t('Previous')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={page >= totalPages || tasksQuery.isFetching}
                onClick={() =>
                  setPage((current) => Math.min(totalPages, current + 1))
                }
              >
                {t('Next')}
                <ChevronRight className='size-3.5' />
              </Button>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

export function AiUnion() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const configQuery = useQuery({
    queryKey: ['ai-union', 'config'],
    queryFn: getAiUnionConfig,
  })
  const config = configQuery.data?.data
  const defaultTokenEnabled = Boolean(config?.default_token.enabled)
  const models = config?.models ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Video Production')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        {config?.default_token && (
          <TokenState
            enabled={defaultTokenEnabled}
            maskedKey={config.default_token.masked_key}
          />
        )}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          {configQuery.isLoading && (
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <Loader2 className='size-4 animate-spin' />
              {t('Loading')}
            </div>
          )}
          {config?.default_token && !defaultTokenEnabled && (
            <div className='border-destructive/30 bg-destructive/5 text-destructive flex flex-wrap items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm'>
              <div className='flex items-center gap-2'>
                <AlertTriangle className='size-4 shrink-0' />
                <span>{t('Default API key is disabled')}</span>
              </div>
              <Button
                type='button'
                size='sm'
                variant='outline'
                onClick={() => void navigate({ to: '/keys' })}
              >
                {t('API Keys')}
              </Button>
            </div>
          )}
          <Tabs defaultValue='workspace'>
            <TabsList className='h-auto max-w-full flex-wrap justify-start'>
              <TabsTrigger value='workspace'>{t('Workspace')}</TabsTrigger>
              <TabsTrigger value='history'>{t('History')}</TabsTrigger>
            </TabsList>
            <TabsContent value='workspace'>
              <WorkspacePanel
                models={models}
                defaultModel={config?.default_model ?? ''}
                disabled={!defaultTokenEnabled}
              />
            </TabsContent>
            <TabsContent value='history'>
              <HistoryPanel />
            </TabsContent>
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
