import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { deleteQueueConfig, getQueueConfigs, upsertQueueConfig } from '../api'
import type { QueueConfig, QueueConfigFormData } from '../types'

const EMPTY_FORM: QueueConfigFormData = {
  model_name: '',
  enabled: true,
  max_queue_size: 0,
  queue_timeout: 0,
  long_context_tiers: [],
}

function normalizeTierValue(value: string): number {
  return Math.max(0, parseInt(value, 10) || 0)
}

export function QueueConfigPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [sheetOpen, setSheetOpen] = useState(false)
  const [editingModel, setEditingModel] = useState<string | null>(null)
  const [formState, setFormState] = useState<QueueConfigFormData>(EMPTY_FORM)

  const queueConfigsQuery = useQuery({
    queryKey: ['queue-configs'],
    queryFn: getQueueConfigs,
  })

  const saveMutation = useMutation({
    mutationFn: async (data: QueueConfigFormData) => {
      return upsertQueueConfig(data.model_name, {
        enabled: data.enabled,
        max_queue_size: data.max_queue_size,
        queue_timeout: data.queue_timeout,
        long_context_tiers: data.long_context_tiers || [],
      })
    },
    onSuccess: () => {
      toast.success(t('Queue configuration saved'))
      setSheetOpen(false)
      queryClient.invalidateQueries({ queryKey: ['queue-configs'] })
      queryClient.invalidateQueries({ queryKey: ['queue-status'] })
    },
    onError: () => {
      toast.error(t('Failed to save queue configuration'))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: async (modelName: string) => deleteQueueConfig(modelName),
    onSuccess: () => {
      toast.success(t('Queue configuration deleted'))
      queryClient.invalidateQueries({ queryKey: ['queue-configs'] })
      queryClient.invalidateQueries({ queryKey: ['queue-status'] })
    },
    onError: () => {
      toast.error(t('Failed to delete queue configuration'))
    },
  })

  const summary = useMemo(() => {
    const configs = queueConfigsQuery.data || []
    return {
      total: configs.length,
      disabled: configs.filter((config) => !config.enabled).length,
      unlimited: configs.filter((config) => config.max_queue_size === 0).length,
      longTierModels: configs.filter(
        (config) => (config.long_context_tiers || []).length > 0
      ).length,
    }
  }, [queueConfigsQuery.data])

  const openCreate = () => {
    setEditingModel(null)
    setFormState(EMPTY_FORM)
    setSheetOpen(true)
  }

  const openEdit = (config: QueueConfig) => {
    setEditingModel(config.model_name)
    setFormState({
      model_name: config.model_name,
      enabled: config.enabled,
      max_queue_size: config.max_queue_size,
      queue_timeout: config.queue_timeout,
      long_context_tiers: config.long_context_tiers || [],
    })
    setSheetOpen(true)
  }

  const updateLongContextTier = (
    index: number,
    patch: Partial<QueueConfigFormData['long_context_tiers'][number]>
  ) => {
    setFormState((current) => ({
      ...current,
      long_context_tiers: current.long_context_tiers.map((tier, tierIndex) =>
        tierIndex === index ? { ...tier, ...patch } : tier
      ),
    }))
  }

  const removeLongContextTier = (index: number) => {
    setFormState((current) => ({
      ...current,
      long_context_tiers: current.long_context_tiers.filter(
        (_tier, tierIndex) => tierIndex !== index
      ),
    }))
  }

  const addLongContextTier = () => {
    setFormState((current) => ({
      ...current,
      long_context_tiers: [
        ...current.long_context_tiers,
        { threshold_tokens: 64000, max_running: 1 },
      ],
    }))
  }

  const handleSave = async () => {
    const modelName = formState.model_name.trim()
    if (!modelName) {
      toast.error(t('Model name is required'))
      return
    }

    const longContextTiers = [...formState.long_context_tiers].sort(
      (left, right) => left.threshold_tokens - right.threshold_tokens
    )
    if (
      longContextTiers.some(
        (tier) => tier.threshold_tokens <= 0 || tier.max_running <= 0
      )
    ) {
      toast.error(t('Long task tier values must be greater than 0'))
      return
    }
    const thresholds = new Set(
      longContextTiers.map((tier) => tier.threshold_tokens)
    )
    if (thresholds.size !== longContextTiers.length) {
      toast.error(t('Long task thresholds must be unique'))
      return
    }
    const hasHigherTierOverLimit = longContextTiers.some((tier, index) => {
      return (
        index > 0 && tier.max_running > longContextTiers[index - 1].max_running
      )
    })
    if (hasHigherTierOverLimit) {
      toast.error(
        t('Long task higher tier max running cannot exceed lower tier')
      )
      return
    }

    await saveMutation.mutateAsync({
      ...formState,
      model_name: modelName,
      max_queue_size: Math.max(0, formState.max_queue_size),
      queue_timeout: Math.max(0, formState.queue_timeout),
      long_context_tiers: longContextTiers,
    })
  }

  const handleDelete = async (modelName: string) => {
    if (
      !confirm(
        t('Are you sure you want to delete the queue config for "{{model}}"?', {
          model: modelName,
        })
      )
    ) {
      return
    }

    await deleteMutation.mutateAsync(modelName)
  }

  if (queueConfigsQuery.isLoading) {
    return <div className='p-4'>{t('Loading...')}</div>
  }

  if (queueConfigsQuery.isError) {
    return <div className='p-4'>{t('Failed to load queue configuration')}</div>
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
        <div>
          <h1 className='text-2xl font-bold'>{t('Queue Config')}</h1>
          <p className='text-muted-foreground text-sm'>
            {t('Override queue enablement, capacity, and timeout per model.')}
          </p>
        </div>
        <div className='flex gap-2'>
          <Button
            variant='outline'
            onClick={() => queueConfigsQuery.refetch()}
            disabled={queueConfigsQuery.isFetching}
          >
            <RefreshCw className='mr-2 h-4 w-4' />
            {t('Refresh')}
          </Button>
          <Button onClick={openCreate}>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add queue config')}
          </Button>
        </div>
      </div>

      <div className='grid gap-4 md:grid-cols-4'>
        <Card>
          <CardHeader>
            <CardDescription>{t('Configured models')}</CardDescription>
            <CardTitle>{summary.total}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t('Disabled overrides')}</CardDescription>
            <CardTitle>{summary.disabled}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t('Unlimited queue limits')}</CardDescription>
            <CardTitle>{summary.unlimited}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t('Long task limits')}</CardDescription>
            <CardTitle>{summary.longTierModels}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('Model queue overrides')}</CardTitle>
          <CardDescription>
            {t('Per-model values override the global request queue settings.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className='rounded-md border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Model')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Max queue size')}</TableHead>
                  <TableHead>{t('Queue timeout')}</TableHead>
                  <TableHead>{t('Long task tiers')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(queueConfigsQuery.data || []).length > 0 ? (
                  (queueConfigsQuery.data || []).map((config) => (
                    <TableRow key={config.model_name}>
                      <TableCell className='font-medium'>
                        {config.model_name}
                      </TableCell>
                      <TableCell>
                        <Badge variant='secondary'>
                          {config.enabled ? t('Enabled') : t('Disabled')}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        {config.max_queue_size === 0
                          ? t('Unlimited')
                          : config.max_queue_size}
                      </TableCell>
                      <TableCell>
                        {config.queue_timeout === 0
                          ? t('System default')
                          : `${config.queue_timeout} ${t('seconds')}`}
                      </TableCell>
                      <TableCell>
                        <div className='flex flex-wrap gap-1'>
                          {(config.long_context_tiers || []).length > 0 ? (
                            (config.long_context_tiers || []).map((tier) => (
                              <Badge
                                key={tier.threshold_tokens}
                                variant='outline'
                              >
                                {`>=${tier.threshold_tokens}: ${tier.max_running}`}
                              </Badge>
                            ))
                          ) : (
                            <span className='text-muted-foreground text-sm'>
                              {t('Not configured')}
                            </span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className='text-right'>
                        <div className='flex justify-end gap-2'>
                          <Button
                            size='sm'
                            variant='outline'
                            onClick={() => openEdit(config)}
                          >
                            <Pencil className='mr-2 h-4 w-4' />
                            {t('Edit')}
                          </Button>
                          <Button
                            size='sm'
                            variant='outline'
                            className='text-destructive'
                            onClick={() => handleDelete(config.model_name)}
                            disabled={deleteMutation.isPending}
                          >
                            <Trash2 className='mr-2 h-4 w-4' />
                            {t('Delete')}
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={6} className='h-24 text-center'>
                      {t('No queue configuration yet')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Sheet
        open={sheetOpen}
        onOpenChange={(open) => {
          setSheetOpen(open)
          if (!open) {
            setEditingModel(null)
            setFormState(EMPTY_FORM)
          }
        }}
      >
        <SheetContent className='sm:max-w-xl'>
          <SheetHeader>
            <SheetTitle>
              {editingModel ? t('Edit queue config') : t('Add queue config')}
            </SheetTitle>
            <SheetDescription>
              {t('Set per-model overrides for the request queue.')}
            </SheetDescription>
          </SheetHeader>

          <div className='min-h-0 flex-1 space-y-4 overflow-y-auto px-4'>
            <div className='space-y-2'>
              <Label htmlFor='queue-model-name'>{t('Model')}</Label>
              <Input
                id='queue-model-name'
                value={formState.model_name}
                disabled={editingModel !== null}
                onChange={(event) =>
                  setFormState((current) => ({
                    ...current,
                    model_name: event.target.value,
                  }))
                }
                placeholder={t('Enter a model name')}
              />
            </div>

            <div className='flex items-center justify-between rounded-lg border p-4'>
              <div className='space-y-1'>
                <p className='text-sm font-medium'>
                  {t('Enable queue for this model')}
                </p>
                <p className='text-muted-foreground text-sm'>
                  {t(
                    'Disabled models bypass queueing and return rate-limit errors immediately.'
                  )}
                </p>
              </div>
              <Switch
                checked={formState.enabled}
                onCheckedChange={(checked) =>
                  setFormState((current) => ({
                    ...current,
                    enabled: checked,
                  }))
                }
              />
            </div>

            <div className='grid gap-4 sm:grid-cols-2'>
              <div className='space-y-2'>
                <Label htmlFor='queue-max-size'>{t('Max queue size')}</Label>
                <Input
                  id='queue-max-size'
                  type='number'
                  min='0'
                  step='1'
                  value={formState.max_queue_size}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      max_queue_size: Math.max(
                        0,
                        parseInt(event.target.value, 10) || 0
                      ),
                    }))
                  }
                />
                <p className='text-muted-foreground text-sm'>
                  {t('0 keeps the queue size unlimited for this model.')}
                </p>
              </div>

              <div className='space-y-2'>
                <Label htmlFor='queue-timeout'>{t('Queue timeout')}</Label>
                <Input
                  id='queue-timeout'
                  type='number'
                  min='0'
                  step='1'
                  value={formState.queue_timeout}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      queue_timeout: Math.max(
                        0,
                        parseInt(event.target.value, 10) || 0
                      ),
                    }))
                  }
                />
                <p className='text-muted-foreground text-sm'>
                  {t('0 uses the global queue timeout.')}
                </p>
              </div>
            </div>

            <div className='space-y-3 rounded-lg border p-4'>
              <div className='flex items-center justify-between gap-3'>
                <div>
                  <p className='text-sm font-medium'>{t('Long task limits')}</p>
                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'Requests at or above a threshold enter the queue and consume the matching running slots.'
                    )}
                  </p>
                </div>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={addLongContextTier}
                >
                  <Plus className='mr-2 h-4 w-4' />
                  {t('Add tier')}
                </Button>
              </div>

              {formState.long_context_tiers.length === 0 ? (
                <div className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>
                  {t('No long context tiers configured.')}
                </div>
              ) : (
                <div className='space-y-3'>
                  {formState.long_context_tiers.map((tier, index) => (
                    <div
                      key={`${tier.threshold_tokens}-${index}`}
                      className='grid gap-3 rounded-md border p-3 sm:grid-cols-[1fr_1fr_auto]'
                    >
                      <div className='space-y-2'>
                        <Label htmlFor={`queue-tier-threshold-${index}`}>
                          {t('Threshold tokens')}
                        </Label>
                        <Input
                          id={`queue-tier-threshold-${index}`}
                          type='number'
                          min='1'
                          step='1'
                          value={tier.threshold_tokens}
                          onChange={(event) =>
                            updateLongContextTier(index, {
                              threshold_tokens: normalizeTierValue(
                                event.target.value
                              ),
                            })
                          }
                        />
                      </div>
                      <div className='space-y-2'>
                        <Label htmlFor={`queue-tier-running-${index}`}>
                          {t('Max running')}
                        </Label>
                        <Input
                          id={`queue-tier-running-${index}`}
                          type='number'
                          min='1'
                          step='1'
                          value={tier.max_running}
                          onChange={(event) =>
                            updateLongContextTier(index, {
                              max_running: normalizeTierValue(
                                event.target.value
                              ),
                            })
                          }
                        />
                      </div>
                      <div className='flex items-end'>
                        <Button
                          type='button'
                          variant='outline'
                          size='icon'
                          className='text-destructive'
                          onClick={() => removeLongContextTier(index)}
                          aria-label={t('Remove tier')}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          <SheetFooter className='mt-6'>
            <Button variant='outline' onClick={() => setSheetOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button onClick={handleSave} disabled={saveMutation.isPending}>
              {saveMutation.isPending ? t('Saving...') : t('Save')}
            </Button>
          </SheetFooter>
        </SheetContent>
      </Sheet>
    </div>
  )
}
