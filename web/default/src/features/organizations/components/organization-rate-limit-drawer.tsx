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
import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Gauge, Pencil, Plus, Trash2, X } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { getModels } from '@/features/models/api'
import {
  createOrganizationRateLimit,
  deleteOrganizationRateLimit,
  getOrganizationRateLimits,
  updateOrganizationRateLimit,
} from '../api'
import type {
  CreateRateLimitRequest,
  OrganizationRateLimit,
} from '../types'
import { OrganizationRateLimitFormDialog } from './organization-rate-limit-form-dialog'

type RateLimitTarget = {
  orgType: 'company' | 'department'
  orgId: number
  orgName: string
}

type OrganizationRateLimitDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  target: RateLimitTarget | null
}

export function OrganizationRateLimitDrawer({
  open,
  onOpenChange,
  target,
}: OrganizationRateLimitDrawerProps) {
  const { t } = useTranslation()
  const [refreshTrigger, setRefreshTrigger] = useState(0)
  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<OrganizationRateLimit | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const { data: rulesData, isLoading } = useQuery({
    queryKey: ['organization-rate-limits', target?.orgType, target?.orgId, refreshTrigger],
    queryFn: () =>
      getOrganizationRateLimits({
        org_type: target!.orgType,
        org_id: target!.orgId,
      }),
    enabled: open && !!target,
  })

  const { data: modelsResponse } = useQuery({
    queryKey: ['organization-rate-limit-models'],
    queryFn: () => getModels({ p: 1, page_size: 100 }),
    enabled: open,
    staleTime: 5 * 60 * 1000,
  })

  const rules = rulesData?.items ?? []

  const modelOptions = useMemo(
    () =>
      (modelsResponse?.data?.items ?? []).map((item) => ({
        value: item.model_name,
        label: item.model_name,
      })),
    [modelsResponse]
  )

  const handleRefresh = () => {
    setRefreshTrigger((prev) => prev + 1)
  }

  const handleCreate = () => {
    setEditingRule(null)
    setIsFormOpen(true)
  }

  const handleEdit = (rule: OrganizationRateLimit) => {
    setEditingRule(rule)
    setIsFormOpen(true)
  }

  const handleDelete = async (rule: OrganizationRateLimit) => {
    if (!confirm(t('Are you sure you want to delete this RPM rule?'))) {
      return
    }

    const result = await deleteOrganizationRateLimit(rule.id)
    if (result.success) {
      toast.success(t('RPM rule deleted successfully'))
      handleRefresh()
      return
    }

    toast.error(result.message || t('Failed to delete RPM rule'))
  }

  const handleSubmit = async (
    payload: CreateRateLimitRequest | Omit<CreateRateLimitRequest, 'org_type' | 'org_id'>
  ) => {
    setIsSubmitting(true)
    try {
      const result = editingRule
        ? await updateOrganizationRateLimit(editingRule.id, payload)
        : await createOrganizationRateLimit(payload as CreateRateLimitRequest)

      if (result.success) {
        toast.success(
          editingRule
            ? t('RPM rule updated successfully')
            : t('RPM rule created successfully')
        )
        setIsFormOpen(false)
        setEditingRule(null)
        handleRefresh()
        return
      }

      toast.error(result.message || t('Operation failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const formatWeekdays = (weekdays?: number[]) => {
    if (!weekdays || weekdays.length === 0) {
      return t('Every day')
    }

    const labels = new Map<number, string>([
      [1, t('Mon')],
      [2, t('Tue')],
      [3, t('Wed')],
      [4, t('Thu')],
      [5, t('Fri')],
      [6, t('Sat')],
      [0, t('Sun')],
    ])

    return weekdays.map((weekday) => labels.get(weekday) || String(weekday)).join(', ')
  }

  const orgTypeLabel =
    target?.orgType === 'company' ? t('Company') : t('Department')

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className='flex w-full flex-col sm:max-w-[760px]'>
          <SheetHeader>
            <div className='flex items-center justify-between'>
              <div className='flex items-center gap-3'>
                <div className='bg-primary/10 text-primary flex h-10 w-10 items-center justify-center rounded-lg'>
                  <Gauge className='h-5 w-5' />
                </div>
                <div>
                  <SheetTitle>{t('RPM Rules')}</SheetTitle>
                  <p className='text-sm text-muted-foreground'>
                    {target?.orgName || ''}
                  </p>
                </div>
              </div>
              <SheetClose render={<Button variant='ghost' size='sm' />}>
                <X className='h-4 w-4' />
                <span className='ml-1'>{t('Close')}</span>
              </SheetClose>
            </div>
          </SheetHeader>

          <div className='mt-4 rounded-xl border bg-muted/30 p-4'>
            <div className='flex items-start justify-between gap-4'>
              <div className='space-y-1'>
                <div className='flex items-center gap-2'>
                  <Badge variant='outline'>{orgTypeLabel}</Badge>
                  <Badge variant='secondary'>
                    {t('{{count}} rules', { count: rules.length })}
                  </Badge>
                </div>
                <p className='text-sm text-muted-foreground'>
                  {t('Configure shared RPM limits for all users in this organization.')}
                </p>
              </div>
              <Button onClick={handleCreate}>
                <Plus className='mr-2 h-4 w-4' />
                {t('Add RPM rule')}
              </Button>
            </div>
          </div>

          <div className='min-h-0 flex-1 py-4'>
            {isLoading ? (
              <div className='flex h-full items-center justify-center text-sm text-muted-foreground'>
                {t('Loading...')}
              </div>
            ) : rules.length === 0 ? (
              <div className='flex h-full flex-col items-center justify-center gap-3 rounded-xl border border-dashed text-center'>
                <Gauge className='h-10 w-10 text-muted-foreground' />
                <div className='space-y-1'>
                  <p className='font-medium'>{t('No RPM rules configured')}</p>
                  <p className='text-sm text-muted-foreground'>
                    {t('Create your first shared RPM rule for this organization.')}
                  </p>
                </div>
              </div>
            ) : (
              <ScrollArea className='h-full pr-3'>
                <div className='space-y-3'>
                  {rules.map((rule) => (
                    <div
                      key={rule.id}
                      className='rounded-xl border border-border/70 p-4 shadow-xs'
                    >
                      <div className='flex flex-wrap items-start justify-between gap-3'>
                        <div className='space-y-2'>
                          <div className='flex flex-wrap items-center gap-2'>
                            <span className='font-medium'>
                              {rule.model_name || t('All Models')}
                            </span>
                            <Badge variant='outline'>
                              {t('Priority')}: {rule.priority}
                            </Badge>
                            <Badge
                              variant='secondary'
                              className={
                                rule.status === 1
                                  ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
                                  : ''
                              }
                            >
                              {rule.status === 1 ? t('Enabled') : t('Disabled')}
                            </Badge>
                          </div>
                          <p className='text-sm text-muted-foreground'>
                            {rule.model_name
                              ? t('Shared RPM applies only to this model.')
                              : t('Shared RPM applies to all models.')}
                          </p>
                        </div>

                        <div className='flex items-center gap-2'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() => handleEdit(rule)}
                          >
                            <Pencil className='mr-2 h-4 w-4' />
                            {t('Edit')}
                          </Button>
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            className='text-destructive'
                            onClick={() => handleDelete(rule)}
                          >
                            <Trash2 className='mr-2 h-4 w-4' />
                            {t('Delete')}
                          </Button>
                        </div>
                      </div>

                      <div className='mt-4 grid gap-2'>
                        {rule.time_slots.map((slot, index) => (
                          <div
                            key={`${rule.id}-${slot.start_time}-${slot.end_time}-${index}`}
                            className='flex flex-col gap-2 rounded-lg bg-muted/40 px-3 py-2 text-sm md:flex-row md:items-center md:justify-between'
                          >
                            <div className='flex flex-wrap items-center gap-2 text-muted-foreground'>
                              <Badge variant='outline'>
                                {formatWeekdays(slot.weekdays)}
                              </Badge>
                              <span>
                                {slot.start_time} - {slot.end_time}
                              </span>
                            </div>
                            <div className='font-medium'>
                              {rule.rpms[index] ?? 0} RPM
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  ))}
                </div>
              </ScrollArea>
            )}
          </div>
        </SheetContent>
      </Sheet>

      {target && (
        <OrganizationRateLimitFormDialog
          open={isFormOpen}
          onOpenChange={(nextOpen) => {
            setIsFormOpen(nextOpen)
            if (!nextOpen) {
              setEditingRule(null)
            }
          }}
          orgType={target.orgType}
          orgId={target.orgId}
          editData={editingRule}
          modelOptions={modelOptions}
          onSubmit={handleSubmit}
          isSubmitting={isSubmitting}
        />
      )}
    </>
  )
}