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
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { z } from 'zod'
import { Minus, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Combobox } from '@/components/ui/combobox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import type {
  CreateRateLimitRequest,
  OrganizationRateLimit,
} from '../types'

const timeSlotSchema = z.object({
  start_time: z.string().min(1),
  end_time: z.string().min(1),
  weekdays: z.array(z.number().int().min(0).max(6)).default([]),
  rpm: z.number().int().min(0),
})

const rateLimitRuleSchema = z.object({
  model_name: z.string().trim().default(''),
  priority: z.number().int().min(0),
  status: z.union([z.literal(0), z.literal(1)]),
  slots: z.array(timeSlotSchema).min(1),
})

type RateLimitRuleFormValues = z.infer<typeof rateLimitRuleSchema>

type ModelOption = {
  value: string
  label: string
}

type OrganizationRateLimitFormDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  orgType: 'company' | 'department'
  orgId: number
  editData?: OrganizationRateLimit | null
  modelOptions: ModelOption[]
  onSubmit: (payload: CreateRateLimitRequest | Omit<CreateRateLimitRequest, 'org_type' | 'org_id'>) => Promise<void>
  isSubmitting: boolean
}

const DEFAULT_VALUES: RateLimitRuleFormValues = {
  model_name: '',
  priority: 0,
  status: 1,
  slots: [
    {
      start_time: '09:00',
      end_time: '18:00',
      weekdays: [],
      rpm: 60,
    },
  ],
}

export function OrganizationRateLimitFormDialog({
  open,
  onOpenChange,
  orgType,
  orgId,
  editData,
  modelOptions,
  onSubmit,
  isSubmitting,
}: OrganizationRateLimitFormDialogProps) {
  const { t } = useTranslation()
  const isEditMode = !!editData

  const form = useForm<RateLimitRuleFormValues>({
    resolver: zodResolver(rateLimitRuleSchema),
    defaultValues: DEFAULT_VALUES,
  })

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'slots',
  })

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

  useEffect(() => {
    if (!open) {
      return
    }

    if (editData) {
      form.reset({
        model_name: editData.model_name ?? '',
        priority: editData.priority ?? 0,
        status: editData.status,
        slots: editData.time_slots.map((slot, index) => ({
          start_time: slot.start_time,
          end_time: slot.end_time,
          weekdays: slot.weekdays ?? [],
          rpm: editData.rpms[index] ?? 0,
        })),
      })
      return
    }

    form.reset(DEFAULT_VALUES)
  }, [editData, form, open])

  const selectedModelName = form.watch('model_name')
  const selectedModel = modelOptions.find(
    (option) => option.value === selectedModelName
  )

  const handleSubmit = async (values: RateLimitRuleFormValues) => {
    const modelName = values.model_name.trim()
    const payload = {
      ...(isEditMode
        ? {}
        : {
            org_type: orgType,
            org_id: orgId,
          }),
          ...(modelName ? { model_name: modelName } : {}),
      priority: values.priority,
      status: values.status,
      time_slots: values.slots.map(({ start_time, end_time, weekdays }) => ({
        start_time,
        end_time,
        weekdays: weekdays.length > 0 ? weekdays : undefined,
      })),
      rpms: values.slots.map((slot) => slot.rpm),
    }

    await onSubmit(payload)
    onOpenChange(false)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-[760px]'>
        <DialogHeader>
          <DialogTitle>
            {isEditMode ? t('Edit RPM rule') : t('Create RPM rule')}
          </DialogTitle>
          <DialogDescription>
            {isEditMode
              ? t('Update the model, time slot, and shared RPM settings for this rule.')
              : t('Create a shared RPM rule for this organization.')}
          </DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(handleSubmit)}
            className='space-y-5'
          >
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='model_name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Model')}</FormLabel>
                    <FormControl>
                      <div className='space-y-2'>
                        {isEditMode ? (
                          <Input
                            value={field.value || ''}
                            readOnly
                            placeholder={t('All Models')}
                          />
                        ) : (
                          <Combobox
                            options={modelOptions}
                            value={field.value ?? ''}
                            onValueChange={(value) => {
                              field.onChange(value ?? '')
                            }}
                            allowCustomValue
                            placeholder={t('Search model name...')}
                            searchPlaceholder={t('Search model name...')}
                            emptyText={t('No model found.')}
                            className='w-full'
                            id='organization-rate-limit-model'
                          />
                        )}
                        <div className='flex items-center gap-2'>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() => field.onChange('')}
                            disabled={isEditMode}
                          >
                            {t('All Models')}
                          </Button>
                          <span className='text-sm text-muted-foreground'>
                            {selectedModel?.label || field.value || t('Rule applies to all models.')}
                          </span>
                        </div>
                      </div>
                    </FormControl>
                    <FormDescription>
                      {isEditMode
                        ? t('Model cannot be changed when editing an existing rule.')
                        : t('Leave empty to share this RPM limit across all models.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='priority'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Priority')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        step={1}
                        {...field}
                        onChange={(event) =>
                          field.onChange(Number(event.target.value) || 0)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Higher priority rules within the same organization are matched first.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Status')}</FormLabel>
                  <Select
                    onValueChange={(value) => field.onChange(Number(value))}
                    value={String(field.value)}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='1'>{t('Enabled')}</SelectItem>
                      <SelectItem value='0'>{t('Disabled')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='space-y-3'>
              <div className='flex items-center justify-between'>
                <div>
                  <h4 className='text-sm font-medium'>{t('Time slots')}</h4>
                  <p className='text-sm text-muted-foreground'>
                    {t('Each time slot defines its own shared RPM limit.')}
                  </p>
                </div>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={() =>
                    append({
                      start_time: '09:00',
                      end_time: '18:00',
                      weekdays: [],
                      rpm: 60,
                    })
                  }
                >
                  <Plus className='mr-2 h-4 w-4' />
                  {t('Add time slot')}
                </Button>
              </div>

              <div className='space-y-4'>
                {fields.map((slotField, index) => (
                  <div
                    key={slotField.id}
                    className='rounded-lg border border-border/70 p-4'
                  >
                    <div className='mb-4 flex items-center justify-between'>
                      <div className='flex items-center gap-2'>
                        <Badge variant='outline'>
                          {t('Slot {{index}}', { index: index + 1 })}
                        </Badge>
                        <span className='text-sm text-muted-foreground'>
                          {t('Leave weekdays empty to apply this slot every day.')}
                        </span>
                      </div>
                      <Button
                        type='button'
                        size='icon'
                        variant='ghost'
                        onClick={() => remove(index)}
                        disabled={fields.length === 1}
                      >
                        <Minus className='h-4 w-4' />
                      </Button>
                    </div>

                    <div className='grid gap-4 md:grid-cols-3'>
                      <FormField
                        control={form.control}
                        name={`slots.${index}.start_time`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('Start time')}</FormLabel>
                            <FormControl>
                              <Input type='time' {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`slots.${index}.end_time`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('End time')}</FormLabel>
                            <FormControl>
                              <Input type='time' {...field} />
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />

                      <FormField
                        control={form.control}
                        name={`slots.${index}.rpm`}
                        render={({ field }) => (
                          <FormItem>
                            <FormLabel>{t('RPM')}</FormLabel>
                            <FormControl>
                              <Input
                                type='number'
                                min={0}
                                step={1}
                                {...field}
                                onChange={(event) =>
                                  field.onChange(Number(event.target.value) || 0)
                                }
                              />
                            </FormControl>
                            <FormDescription>
                              {t('Shared requests per minute for this slot.')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                    </div>

                    <FormField
                      control={form.control}
                      name={`slots.${index}.weekdays`}
                      render={({ field }) => {
                        const value = field.value ?? []
                        return (
                          <FormItem className='mt-4'>
                            <FormLabel>{t('Weekdays')}</FormLabel>
                            <FormControl>
                              <div className='flex flex-wrap gap-3'>
                                {weekdayOptions.map((option) => {
                                  const checked = value.includes(option.value)
                                  return (
                                    <label
                                      key={option.value}
                                      className='flex items-center gap-2 rounded-md border px-3 py-2 text-sm'
                                    >
                                      <Checkbox
                                        checked={checked}
                                        onCheckedChange={(nextChecked) => {
                                          if (nextChecked) {
                                            field.onChange(
                                              [...value, option.value].sort((a, b) => a - b)
                                            )
                                            return
                                          }

                                          field.onChange(
                                            value.filter((day) => day !== option.value)
                                          )
                                        }}
                                      />
                                      <span>{option.label}</span>
                                    </label>
                                  )
                                })}
                              </div>
                            </FormControl>
                            <FormDescription>
                              {t('Leave all unchecked to make this slot active every day.')}
                            </FormDescription>
                            <FormMessage />
                          </FormItem>
                        )
                      }}
                    />
                  </div>
                ))}
              </div>
            </div>

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={isSubmitting}>
                {isEditMode ? t('Update') : t('Add')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}