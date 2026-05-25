import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useForm, useFieldArray } from 'react-hook-form'
import { toast } from 'sonner'
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
import { Button } from '@/components/ui/button'
import { Plus, Trash2 } from 'lucide-react'
import { createRateLimit, updateRateLimit } from '../api'
import type { RateLimitRule, RateLimitFormValues, TimeSlot } from '../types'

interface OrganizationRateLimitFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  orgType: string
  orgId: number
  editingRule: RateLimitRule | null
  onSuccess: () => void
}

const WEEKDAY_OPTIONS = [
  { value: 0, label: 'Sun' },
  { value: 1, label: 'Mon' },
  { value: 2, label: 'Tue' },
  { value: 3, label: 'Wed' },
  { value: 4, label: 'Thu' },
  { value: 5, label: 'Fri' },
  { value: 6, label: 'Sat' },
]

interface FormValues {
  model_name: string
  priority: number
  status: number
  slots: {
    start_time: string
    end_time: string
    weekdays: number[]
    rpm: number
  }[]
}

export function OrganizationRateLimitFormDialog({
  open,
  onOpenChange,
  orgType,
  orgId,
  editingRule,
  onSuccess,
}: OrganizationRateLimitFormDialogProps) {
  const { t } = useTranslation()
  const isEditing = !!editingRule

  const form = useForm<FormValues>({
    defaultValues: {
      model_name: '',
      priority: 0,
      status: 1,
      slots: [{ start_time: '00:00', end_time: '23:59', weekdays: [], rpm: 60 }],
    },
  })

  const { fields, append, remove } = useFieldArray({
    control: form.control,
    name: 'slots',
  })

  useEffect(() => {
    if (open && editingRule) {
      const slots = editingRule.time_slots.map((slot, i) => ({
        start_time: slot.start_time,
        end_time: slot.end_time,
        weekdays: slot.weekdays || [],
        rpm: editingRule.rpms[i] || 0,
      }))
      form.reset({
        model_name: editingRule.model_name || '',
        priority: editingRule.priority,
        status: editingRule.status,
        slots: slots.length > 0 ? slots : [{ start_time: '00:00', end_time: '23:59', weekdays: [], rpm: 60 }],
      })
    } else if (open) {
      form.reset({
        model_name: '',
        priority: 0,
        status: 1,
        slots: [{ start_time: '00:00', end_time: '23:59', weekdays: [], rpm: 60 }],
      })
    }
  }, [open, editingRule, form])

  const onSubmit = async (data: FormValues) => {
    try {
      const time_slots: TimeSlot[] = data.slots.map((s) => ({
        start_time: s.start_time,
        end_time: s.end_time,
        weekdays: s.weekdays,
      }))
      const rpms = data.slots.map((s) => s.rpm)

      if (isEditing && editingRule) {
        const res = await updateRateLimit(editingRule.id, {
          time_slots,
          rpms,
          priority: data.priority,
          status: data.status,
        })
        if (res.success) {
          toast.success(t('Rule updated successfully'))
          onSuccess()
        } else {
          toast.error(res.message || t('Failed to update rule'))
        }
      } else {
        const payload: RateLimitFormValues = {
          org_type: orgType,
          org_id: orgId,
          model_name: data.model_name || undefined,
          time_slots,
          rpms,
          priority: data.priority,
          status: data.status,
        }
        const res = await createRateLimit(payload)
        if (res.success) {
          toast.success(t('Rule created successfully'))
          onSuccess()
        } else {
          toast.error(res.message || t('Failed to create rule'))
        }
      }
    } catch {
      toast.error(t('Failed to save rule'))
    }
  }

  const toggleWeekday = (slotIndex: number, day: number) => {
    const current = form.getValues(`slots.${slotIndex}.weekdays`) || []
    const updated = current.includes(day)
      ? current.filter((d: number) => d !== day)
      : [...current, day]
    form.setValue(`slots.${slotIndex}.weekdays`, updated)
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='sm:max-w-lg max-h-[80vh] overflow-y-auto'>
        <DialogHeader>
          <DialogTitle>
            {isEditing ? t('Edit RPM Rule') : t('New RPM Rule')}
          </DialogTitle>
          <DialogDescription>
            {isEditing
              ? t('Update the rate limit rule configuration')
              : t('Create a new rate limit rule')}
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            {/* Model Name */}
            <FormField
              control={form.control}
              name='model_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Model')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder={t('All Models')}
                      disabled={isEditing}
                    />
                  </FormControl>
                  {isEditing && (
                    <p className='text-xs text-muted-foreground'>
                      {t('Model cannot be changed after creation')}
                    </p>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Priority */}
            <FormField
              control={form.control}
              name='priority'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Priority')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      {...field}
                      onChange={(e) => field.onChange(Number(e.target.value))}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Status */}
            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Status')}</FormLabel>
                  <Select
                    onValueChange={(v) => field.onChange(Number(v))}
                    value={String(field.value)}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='1'>{t('Enabled')}</SelectItem>
                      <SelectItem value='2'>{t('Disabled')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Time Slots */}
            <div className='space-y-3'>
              <div className='flex items-center justify-between'>
                <FormLabel>{t('Time Slots')}</FormLabel>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    append({ start_time: '00:00', end_time: '23:59', weekdays: [], rpm: 60 })
                  }
                >
                  <Plus />
                  {t('Add Slot')}
                </Button>
              </div>

              {fields.map((field, index) => (
                <div key={field.id} className='rounded-md border p-3 space-y-2'>
                  <div className='flex items-center justify-between'>
                    <span className='text-sm font-medium'>
                      {t('Slot')} {index + 1}
                    </span>
                    {fields.length > 1 && (
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => remove(index)}
                      >
                        <Trash2 className='h-3.5 w-3.5 text-destructive' />
                      </Button>
                    )}
                  </div>

                  <div className='grid grid-cols-2 gap-2'>
                    <FormField
                      control={form.control}
                      name={`slots.${index}.start_time`}
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel className='text-xs'>
                            {t('Start Time')}
                          </FormLabel>
                          <FormControl>
                            <Input {...field} placeholder='HH:MM' />
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
                          <FormLabel className='text-xs'>
                            {t('End Time')}
                          </FormLabel>
                          <FormControl>
                            <Input {...field} placeholder='HH:MM' />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>

                  <FormField
                    control={form.control}
                    name={`slots.${index}.rpm`}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel className='text-xs'>RPM</FormLabel>
                        <FormControl>
                          <Input
                            type='number'
                            {...field}
                            onChange={(e) =>
                              field.onChange(Number(e.target.value))
                            }
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />

                  {/* Weekdays */}
                  <div>
                    <FormLabel className='text-xs'>{t('Weekdays')}</FormLabel>
                    <p className='text-xs text-muted-foreground mb-1'>
                      {t('Leave empty for every day')}
                    </p>
                    <div className='flex flex-wrap gap-1'>
                      {WEEKDAY_OPTIONS.map((day) => {
                        const selected =
                          (form.watch(`slots.${index}.weekdays`) || []).includes(
                            day.value
                          )
                        return (
                          <Button
                            key={day.value}
                            type='button'
                            variant={selected ? 'default' : 'outline'}
                            size='sm'
                            className='h-7 px-2 text-xs'
                            onClick={() => toggleWeekday(index, day.value)}
                          >
                            {t(day.label)}
                          </Button>
                        )
                      })}
                    </div>
                  </div>
                </div>
              ))}
            </div>

            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit'>
                {isEditing ? t('Save Changes') : t('Create')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
