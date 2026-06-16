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
import { Checkbox } from '@/components/ui/checkbox'
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
import type {
  QueueConfig,
  QueueConfigFormData,
  QueueLongContextTier,
  QueueTimeSlotConfig,
} from '../types'

const DEFAULT_QUEUE_LEASE_TURNS = 20
const DEFAULT_QUEUE_LEASE_IDLE_TIMEOUT_SECONDS = 10
type QueueScheduleMode = 'always' | 'scheduled'

const EMPTY_FORM: QueueConfigFormData = {
  model_name: '',
  enabled: true,
  max_queue_size: 0,
  queue_timeout: 0,
  long_context_tiers: [],
  time_slots: [],
}

function normalizeTierValue(value: string): number {
  return Math.max(0, parseInt(value, 10) || 0)
}

function normalizeLongContextTierForForm(
  tier: Partial<QueueLongContextTier>
): QueueLongContextTier {
  return {
    threshold_tokens: tier.threshold_tokens ?? 64000,
    max_running: tier.max_running ?? 1,
    lease_turns: tier.lease_turns || DEFAULT_QUEUE_LEASE_TURNS,
    lease_idle_timeout_seconds:
      tier.lease_idle_timeout_seconds ||
      DEFAULT_QUEUE_LEASE_IDLE_TIMEOUT_SECONDS,
  }
}

function normalizeTimeSlotForForm(
  slot: Partial<QueueTimeSlotConfig>
): QueueTimeSlotConfig {
  return {
    start_time: slot.start_time || '09:00',
    end_time: slot.end_time || '18:00',
    weekdays: slot.weekdays || [],
    enabled: slot.enabled ?? true,
    max_queue_size: slot.max_queue_size ?? 0,
    queue_timeout: slot.queue_timeout ?? 0,
    long_context_tiers: (slot.long_context_tiers || []).map(
      normalizeLongContextTierForForm
    ),
  }
}

function getQueueScheduleMode(config: Pick<QueueConfig, 'time_slots'>) {
  return (config.time_slots || []).length > 0 ? 'scheduled' : 'always'
}

function hasLongTaskLimits(config: QueueConfig) {
  if (getQueueScheduleMode(config) === 'scheduled') {
    return (config.time_slots || []).some(
      (slot) => (slot.long_context_tiers || []).length > 0
    )
  }
  return (config.long_context_tiers || []).length > 0
}

function hasUnlimitedQueueLimit(config: QueueConfig) {
  if (getQueueScheduleMode(config) === 'scheduled') {
    return (config.time_slots || []).some((slot) => slot.max_queue_size === 0)
  }
  return config.max_queue_size === 0
}

function isEffectivelyDisabled(config: QueueConfig) {
  if (getQueueScheduleMode(config) === 'scheduled') {
    return (config.time_slots || []).every((slot) => !slot.enabled)
  }
  return !config.enabled
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
        time_slots: data.time_slots || [],
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
      disabled: configs.filter(isEffectivelyDisabled).length,
      unlimited: configs.filter(hasUnlimitedQueueLimit).length,
      longTierModels: configs.filter(hasLongTaskLimits).length,
      timeSlotModels: configs.filter(
        (config) => (config.time_slots || []).length > 0
      ).length,
    }
  }, [queueConfigsQuery.data])

  const weekdayOptions = useMemo(
    () => [
      { value: 1, label: t('Mon') },
      { value: 2, label: t('Tue') },
      { value: 3, label: t('Wed') },
      { value: 4, label: t('Thu') },
      { value: 5, label: t('Fri') },
      { value: 6, label: t('Sat') },
      { value: 0, label: t('Sun') },
    ],
    [t]
  )

  const scheduleMode: QueueScheduleMode =
    formState.time_slots.length > 0 ? 'scheduled' : 'always'

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
      long_context_tiers: (config.long_context_tiers || []).map(
        normalizeLongContextTierForForm
      ),
      time_slots: (config.time_slots || []).map(normalizeTimeSlotForForm),
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
        normalizeLongContextTierForForm({}),
      ],
    }))
  }

  const updateScheduleMode = (mode: QueueScheduleMode) => {
    setFormState((current) => {
      if (mode === 'always') {
        return {
          ...current,
          time_slots: [],
        }
      }
      return {
        ...current,
        time_slots:
          current.time_slots.length > 0
            ? current.time_slots
            : [normalizeTimeSlotForForm({})],
      }
    })
  }

  const updateTimeSlot = (
    index: number,
    patch: Partial<QueueConfigFormData['time_slots'][number]>
  ) => {
    setFormState((current) => ({
      ...current,
      time_slots: current.time_slots.map((slot, slotIndex) =>
        slotIndex === index ? { ...slot, ...patch } : slot
      ),
    }))
  }

  const removeTimeSlot = (index: number) => {
    setFormState((current) => ({
      ...current,
      time_slots: current.time_slots.filter(
        (_slot, slotIndex) => slotIndex !== index
      ),
    }))
  }

  const addTimeSlot = () => {
    setFormState((current) => ({
      ...current,
      time_slots: [...current.time_slots, normalizeTimeSlotForForm({})],
    }))
  }

  const updateTimeSlotTier = (
    slotIndex: number,
    tierIndex: number,
    patch: Partial<QueueLongContextTier>
  ) => {
    setFormState((current) => ({
      ...current,
      time_slots: current.time_slots.map((slot, currentSlotIndex) => {
        if (currentSlotIndex !== slotIndex) {
          return slot
        }
        return {
          ...slot,
          long_context_tiers: slot.long_context_tiers.map(
            (tier, currentTierIndex) =>
              currentTierIndex === tierIndex ? { ...tier, ...patch } : tier
          ),
        }
      }),
    }))
  }

  const addTimeSlotTier = (slotIndex: number) => {
    setFormState((current) => ({
      ...current,
      time_slots: current.time_slots.map((slot, currentSlotIndex) =>
        currentSlotIndex === slotIndex
          ? {
              ...slot,
              long_context_tiers: [
                ...slot.long_context_tiers,
                normalizeLongContextTierForForm({}),
              ],
            }
          : slot
      ),
    }))
  }

  const removeTimeSlotTier = (slotIndex: number, tierIndex: number) => {
    setFormState((current) => ({
      ...current,
      time_slots: current.time_slots.map((slot, currentSlotIndex) =>
        currentSlotIndex === slotIndex
          ? {
              ...slot,
              long_context_tiers: slot.long_context_tiers.filter(
                (_tier, currentTierIndex) => currentTierIndex !== tierIndex
              ),
            }
          : slot
      ),
    }))
  }

  const validateLongContextTiers = (tiers: QueueLongContextTier[]) => {
    const longContextTiers = [...tiers].sort(
      (left, right) => left.threshold_tokens - right.threshold_tokens
    )
    if (
      longContextTiers.some(
        (tier) =>
          tier.threshold_tokens <= 0 ||
          tier.max_running <= 0 ||
          tier.lease_turns <= 0 ||
          tier.lease_idle_timeout_seconds <= 0
      )
    ) {
      return t('Long task tier values must be greater than 0')
    }
    const thresholds = new Set(
      longContextTiers.map((tier) => tier.threshold_tokens)
    )
    if (thresholds.size !== longContextTiers.length) {
      return t('Long task thresholds must be unique')
    }
    const hasHigherTierOverLimit = longContextTiers.some((tier, index) => {
      return (
        index > 0 && tier.max_running > longContextTiers[index - 1].max_running
      )
    })
    if (hasHigherTierOverLimit) {
      return t('Long task higher tier max running cannot exceed lower tier')
    }
    return null
  }

  const handleSave = async () => {
    const modelName = formState.model_name.trim()
    if (!modelName) {
      toast.error(t('Model name is required'))
      return
    }

    const isScheduledMode = formState.time_slots.length > 0
    const longContextTiers: QueueLongContextTier[] = isScheduledMode
      ? []
      : [...formState.long_context_tiers].sort(
          (left, right) => left.threshold_tokens - right.threshold_tokens
        )
    if (!isScheduledMode) {
      const longContextError = validateLongContextTiers(longContextTiers)
      if (longContextError) {
        toast.error(longContextError)
        return
      }
    }
    const timeSlots = isScheduledMode
      ? formState.time_slots.map((slot) => ({
          ...slot,
          max_queue_size: Math.max(0, slot.max_queue_size),
          queue_timeout: Math.max(0, slot.queue_timeout),
          long_context_tiers: [...slot.long_context_tiers].sort(
            (left, right) => left.threshold_tokens - right.threshold_tokens
          ),
        }))
      : []
    for (const slot of timeSlots) {
      if (!slot.start_time || !slot.end_time) {
        toast.error(t('Queue time slot times are required'))
        return
      }
      const slotLongContextError = validateLongContextTiers(
        slot.long_context_tiers
      )
      if (slotLongContextError) {
        toast.error(slotLongContextError)
        return
      }
    }

    await saveMutation.mutateAsync({
      ...formState,
      model_name: modelName,
      enabled: isScheduledMode ? true : formState.enabled,
      max_queue_size: isScheduledMode
        ? 0
        : Math.max(0, formState.max_queue_size),
      queue_timeout: isScheduledMode ? 0 : Math.max(0, formState.queue_timeout),
      long_context_tiers: longContextTiers,
      time_slots: timeSlots,
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

      <div className='grid gap-4 md:grid-cols-5'>
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
        <Card>
          <CardHeader>
            <CardDescription>{t('Time-slotted models')}</CardDescription>
            <CardTitle>{summary.timeSlotModels}</CardTitle>
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
                  <TableHead>{t('Time slots')}</TableHead>
                  <TableHead>{t('Long task tiers')}</TableHead>
                  <TableHead className='text-right'>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {(queueConfigsQuery.data || []).length > 0 ? (
                  (queueConfigsQuery.data || []).map((config) => {
                    const isScheduled =
                      getQueueScheduleMode(config) === 'scheduled'
                    const enabledSlots = (config.time_slots || []).filter(
                      (slot) => slot.enabled
                    )
                    const slotLongTaskLimits = (config.time_slots || []).filter(
                      (slot) => (slot.long_context_tiers || []).length > 0
                    )

                    return (
                      <TableRow key={config.model_name}>
                        <TableCell className='font-medium'>
                          {config.model_name}
                        </TableCell>
                        <TableCell>
                          <Badge variant='secondary'>
                            {isScheduled
                              ? enabledSlots.length > 0
                                ? t('Scheduled')
                                : t('Disabled')
                              : config.enabled
                                ? t('Enabled')
                                : t('Disabled')}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          {isScheduled ? (
                            <span className='text-muted-foreground text-sm'>
                              {t('Configured per time slot')}
                            </span>
                          ) : config.max_queue_size === 0 ? (
                            t('Unlimited')
                          ) : (
                            config.max_queue_size
                          )}
                        </TableCell>
                        <TableCell>
                          {isScheduled ? (
                            <span className='text-muted-foreground text-sm'>
                              {t('Configured per time slot')}
                            </span>
                          ) : config.queue_timeout === 0 ? (
                            t('System default')
                          ) : (
                            `${config.queue_timeout} ${t('seconds')}`
                          )}
                        </TableCell>
                        <TableCell>
                          <div className='flex flex-wrap gap-1'>
                            {isScheduled ? (
                              (config.time_slots || []).map((slot, index) => (
                                <Badge
                                  key={`${slot.start_time}-${slot.end_time}-${index}`}
                                  variant='outline'
                                >
                                  {`${slot.start_time}-${slot.end_time} · ${slot.enabled ? t('Enabled') : t('Disabled')}`}
                                </Badge>
                              ))
                            ) : (
                              <span className='text-muted-foreground text-sm'>
                                {t('Always active')}
                              </span>
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <div className='flex flex-wrap gap-1'>
                            {isScheduled ? (
                              slotLongTaskLimits.length > 0 ? (
                                slotLongTaskLimits.map((slot, index) => (
                                  <Badge
                                    key={`${slot.start_time}-${slot.end_time}-${index}`}
                                    variant='outline'
                                  >
                                    {`${slot.start_time}-${slot.end_time}: ${slot.long_context_tiers.length} ${t('tiers')}`}
                                  </Badge>
                                ))
                              ) : (
                                <span className='text-muted-foreground text-sm'>
                                  {t('Not configured')}
                                </span>
                              )
                            ) : (config.long_context_tiers || []).length > 0 ? (
                              (config.long_context_tiers || [])
                                .map(normalizeLongContextTierForForm)
                                .map((tier) => (
                                  <Badge
                                    key={tier.threshold_tokens}
                                    variant='outline'
                                  >
                                    {`>=${tier.threshold_tokens}: ${tier.max_running} · ${tier.lease_turns} ${t('turns')} · ${tier.lease_idle_timeout_seconds}s`}
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
                    )
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={7} className='h-24 text-center'>
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
        <SheetContent className='sm:max-w-3xl'>
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

            <div className='space-y-2'>
              <Label>{t('Effective range')}</Label>
              <div
                role='group'
                aria-label={t('Effective range')}
                className='bg-muted/60 inline-flex rounded-lg border p-0.5'
              >
                <button
                  type='button'
                  aria-pressed={scheduleMode === 'always'}
                  className={`h-8 rounded-md px-3 text-sm font-medium transition-colors ${
                    scheduleMode === 'always'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  onClick={() => updateScheduleMode('always')}
                >
                  {t('All-day')}
                </button>
                <button
                  type='button'
                  aria-pressed={scheduleMode === 'scheduled'}
                  className={`h-8 rounded-md px-3 text-sm font-medium transition-colors ${
                    scheduleMode === 'scheduled'
                      ? 'bg-primary text-primary-foreground shadow-sm'
                      : 'text-muted-foreground hover:text-foreground'
                  }`}
                  onClick={() => updateScheduleMode('scheduled')}
                >
                  {t('Time slots')}
                </button>
              </div>
              <p className='text-muted-foreground text-sm'>
                {scheduleMode === 'always'
                  ? t('Static queue settings stay active all day.')
                  : t(
                      'Each matching time slot uses its own queue size, timeout, and long task limits.'
                    )}
              </p>
            </div>

            {scheduleMode === 'always' && (
              <>
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
                    <Label htmlFor='queue-max-size'>
                      {t('Max queue size')}
                    </Label>
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
                      <p className='text-sm font-medium'>
                        {t('All-day long task limits')}
                      </p>
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
                          className='grid gap-3 rounded-md border p-3 sm:grid-cols-2'
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
                          <div className='space-y-2'>
                            <Label htmlFor={`queue-tier-lease-turns-${index}`}>
                              {t('Lease turns')}
                            </Label>
                            <Input
                              id={`queue-tier-lease-turns-${index}`}
                              type='number'
                              min='1'
                              step='1'
                              value={tier.lease_turns}
                              onChange={(event) =>
                                updateLongContextTier(index, {
                                  lease_turns: normalizeTierValue(
                                    event.target.value
                                  ),
                                })
                              }
                            />
                          </div>
                          <div className='space-y-2'>
                            <Label htmlFor={`queue-tier-lease-idle-${index}`}>
                              {t('Idle release seconds')}
                            </Label>
                            <Input
                              id={`queue-tier-lease-idle-${index}`}
                              type='number'
                              min='1'
                              step='1'
                              value={tier.lease_idle_timeout_seconds}
                              onChange={(event) =>
                                updateLongContextTier(index, {
                                  lease_idle_timeout_seconds:
                                    normalizeTierValue(event.target.value),
                                })
                              }
                            />
                          </div>
                          <div className='flex items-end sm:col-span-2'>
                            <Button
                              type='button'
                              variant='outline'
                              className='text-destructive'
                              onClick={() => removeLongContextTier(index)}
                              aria-label={t('Remove tier')}
                            >
                              <Trash2 className='mr-2 h-4 w-4' />
                              {t('Remove tier')}
                            </Button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </>
            )}

            {scheduleMode === 'scheduled' && (
              <div className='space-y-3 rounded-lg border p-4'>
                <div className='flex items-center justify-between gap-3'>
                  <div>
                    <p className='text-sm font-medium'>
                      {t('Queue time slots')}
                    </p>
                    <p className='text-muted-foreground text-sm'>
                      {t('Queueing is disabled outside matching time slots.')}
                    </p>
                  </div>
                  <Button
                    type='button'
                    size='sm'
                    variant='outline'
                    onClick={addTimeSlot}
                  >
                    <Plus className='mr-2 h-4 w-4' />
                    {t('Add time slot')}
                  </Button>
                </div>

                {formState.time_slots.length === 0 ? (
                  <div className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>
                    {t(
                      'No queue time slots configured. Static settings are always active.'
                    )}
                  </div>
                ) : (
                  <div className='space-y-4'>
                    {formState.time_slots.map((slot, slotIndex) => (
                      <div
                        key={`${slot.start_time}-${slot.end_time}-${slotIndex}`}
                        className='space-y-4 rounded-md border p-3'
                      >
                        <div className='flex flex-wrap items-center justify-between gap-3'>
                          <Badge variant='outline'>
                            {t('Slot {{index}}', { index: slotIndex + 1 })}
                          </Badge>
                          <Button
                            type='button'
                            variant='outline'
                            className='text-destructive'
                            onClick={() => removeTimeSlot(slotIndex)}
                          >
                            <Trash2 className='mr-2 h-4 w-4' />
                            {t('Remove time slot')}
                          </Button>
                        </div>

                        <div className='grid gap-4 sm:grid-cols-2'>
                          <div className='space-y-2'>
                            <Label htmlFor={`queue-slot-start-${slotIndex}`}>
                              {t('Start time')}
                            </Label>
                            <Input
                              id={`queue-slot-start-${slotIndex}`}
                              type='time'
                              value={slot.start_time}
                              onChange={(event) =>
                                updateTimeSlot(slotIndex, {
                                  start_time: event.target.value,
                                })
                              }
                            />
                          </div>
                          <div className='space-y-2'>
                            <Label htmlFor={`queue-slot-end-${slotIndex}`}>
                              {t('End time')}
                            </Label>
                            <Input
                              id={`queue-slot-end-${slotIndex}`}
                              type='time'
                              value={slot.end_time}
                              onChange={(event) =>
                                updateTimeSlot(slotIndex, {
                                  end_time: event.target.value,
                                })
                              }
                            />
                          </div>
                        </div>

                        <div className='space-y-2'>
                          <Label>{t('Weekdays')}</Label>
                          <div className='flex flex-wrap gap-2'>
                            {weekdayOptions.map((option) => {
                              const weekdays = slot.weekdays || []
                              const checked = weekdays.includes(option.value)
                              return (
                                <label
                                  key={option.value}
                                  className='flex items-center gap-2 rounded-md border px-3 py-2 text-sm'
                                >
                                  <Checkbox
                                    checked={checked}
                                    onCheckedChange={(nextChecked) => {
                                      if (nextChecked) {
                                        updateTimeSlot(slotIndex, {
                                          weekdays: [
                                            ...weekdays,
                                            option.value,
                                          ].sort((left, right) => left - right),
                                        })
                                        return
                                      }
                                      updateTimeSlot(slotIndex, {
                                        weekdays: weekdays.filter(
                                          (day) => day !== option.value
                                        ),
                                      })
                                    }}
                                  />
                                  <span>{option.label}</span>
                                </label>
                              )
                            })}
                          </div>
                          <p className='text-muted-foreground text-sm'>
                            {t(
                              'Leave all unchecked to make this slot active every day.'
                            )}
                          </p>
                        </div>

                        <div className='flex items-center justify-between rounded-md border p-3'>
                          <div>
                            <p className='text-sm font-medium'>
                              {t('Enable queue in this slot')}
                            </p>
                            <p className='text-muted-foreground text-sm'>
                              {t(
                                'Disabled slots match the time window but do not allow queueing.'
                              )}
                            </p>
                          </div>
                          <Switch
                            checked={slot.enabled}
                            onCheckedChange={(checked) =>
                              updateTimeSlot(slotIndex, { enabled: checked })
                            }
                          />
                        </div>

                        <div className='grid gap-4 sm:grid-cols-2'>
                          <div className='space-y-2'>
                            <Label htmlFor={`queue-slot-max-size-${slotIndex}`}>
                              {t('Max queue size')}
                            </Label>
                            <Input
                              id={`queue-slot-max-size-${slotIndex}`}
                              type='number'
                              min='0'
                              step='1'
                              value={slot.max_queue_size}
                              onChange={(event) =>
                                updateTimeSlot(slotIndex, {
                                  max_queue_size: Math.max(
                                    0,
                                    parseInt(event.target.value, 10) || 0
                                  ),
                                })
                              }
                            />
                          </div>
                          <div className='space-y-2'>
                            <Label htmlFor={`queue-slot-timeout-${slotIndex}`}>
                              {t('Queue timeout')}
                            </Label>
                            <Input
                              id={`queue-slot-timeout-${slotIndex}`}
                              type='number'
                              min='0'
                              step='1'
                              value={slot.queue_timeout}
                              onChange={(event) =>
                                updateTimeSlot(slotIndex, {
                                  queue_timeout: Math.max(
                                    0,
                                    parseInt(event.target.value, 10) || 0
                                  ),
                                })
                              }
                            />
                          </div>
                        </div>

                        <div className='space-y-3 rounded-md border p-3'>
                          <div className='flex items-center justify-between gap-3'>
                            <div>
                              <p className='text-sm font-medium'>
                                {t('Long task limits')}
                              </p>
                              <p className='text-muted-foreground text-sm'>
                                {t(
                                  'These long task tiers apply only inside this time slot.'
                                )}
                              </p>
                            </div>
                            <Button
                              type='button'
                              size='sm'
                              variant='outline'
                              onClick={() => addTimeSlotTier(slotIndex)}
                            >
                              <Plus className='mr-2 h-4 w-4' />
                              {t('Add tier')}
                            </Button>
                          </div>

                          {slot.long_context_tiers.length === 0 ? (
                            <div className='text-muted-foreground rounded-md border border-dashed p-4 text-sm'>
                              {t('No long context tiers configured.')}
                            </div>
                          ) : (
                            <div className='space-y-3'>
                              {slot.long_context_tiers.map(
                                (tier, tierIndex) => (
                                  <div
                                    key={`${slotIndex}-${tier.threshold_tokens}-${tierIndex}`}
                                    className='grid gap-3 rounded-md border p-3 sm:grid-cols-2'
                                  >
                                    <div className='space-y-2'>
                                      <Label
                                        htmlFor={`queue-slot-tier-threshold-${slotIndex}-${tierIndex}`}
                                      >
                                        {t('Threshold tokens')}
                                      </Label>
                                      <Input
                                        id={`queue-slot-tier-threshold-${slotIndex}-${tierIndex}`}
                                        type='number'
                                        min='1'
                                        step='1'
                                        value={tier.threshold_tokens}
                                        onChange={(event) =>
                                          updateTimeSlotTier(
                                            slotIndex,
                                            tierIndex,
                                            {
                                              threshold_tokens:
                                                normalizeTierValue(
                                                  event.target.value
                                                ),
                                            }
                                          )
                                        }
                                      />
                                    </div>
                                    <div className='space-y-2'>
                                      <Label
                                        htmlFor={`queue-slot-tier-running-${slotIndex}-${tierIndex}`}
                                      >
                                        {t('Max running')}
                                      </Label>
                                      <Input
                                        id={`queue-slot-tier-running-${slotIndex}-${tierIndex}`}
                                        type='number'
                                        min='1'
                                        step='1'
                                        value={tier.max_running}
                                        onChange={(event) =>
                                          updateTimeSlotTier(
                                            slotIndex,
                                            tierIndex,
                                            {
                                              max_running: normalizeTierValue(
                                                event.target.value
                                              ),
                                            }
                                          )
                                        }
                                      />
                                    </div>
                                    <div className='space-y-2'>
                                      <Label
                                        htmlFor={`queue-slot-tier-turns-${slotIndex}-${tierIndex}`}
                                      >
                                        {t('Lease turns')}
                                      </Label>
                                      <Input
                                        id={`queue-slot-tier-turns-${slotIndex}-${tierIndex}`}
                                        type='number'
                                        min='1'
                                        step='1'
                                        value={tier.lease_turns}
                                        onChange={(event) =>
                                          updateTimeSlotTier(
                                            slotIndex,
                                            tierIndex,
                                            {
                                              lease_turns: normalizeTierValue(
                                                event.target.value
                                              ),
                                            }
                                          )
                                        }
                                      />
                                    </div>
                                    <div className='space-y-2'>
                                      <Label
                                        htmlFor={`queue-slot-tier-idle-${slotIndex}-${tierIndex}`}
                                      >
                                        {t('Idle release seconds')}
                                      </Label>
                                      <Input
                                        id={`queue-slot-tier-idle-${slotIndex}-${tierIndex}`}
                                        type='number'
                                        min='1'
                                        step='1'
                                        value={tier.lease_idle_timeout_seconds}
                                        onChange={(event) =>
                                          updateTimeSlotTier(
                                            slotIndex,
                                            tierIndex,
                                            {
                                              lease_idle_timeout_seconds:
                                                normalizeTierValue(
                                                  event.target.value
                                                ),
                                            }
                                          )
                                        }
                                      />
                                    </div>
                                    <div className='flex items-end sm:col-span-2'>
                                      <Button
                                        type='button'
                                        variant='outline'
                                        className='text-destructive'
                                        onClick={() =>
                                          removeTimeSlotTier(
                                            slotIndex,
                                            tierIndex
                                          )
                                        }
                                        aria-label={t('Remove tier')}
                                      >
                                        <Trash2 className='mr-2 h-4 w-4' />
                                        {t('Remove tier')}
                                      </Button>
                                    </div>
                                  </div>
                                )
                              )}
                            </div>
                          )}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}
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
