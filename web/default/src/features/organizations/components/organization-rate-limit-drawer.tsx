import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { getRateLimits, deleteRateLimit } from '../api'
import type { Company, Department, RateLimitRule } from '../types'
import { OrganizationRateLimitFormDialog } from './organization-rate-limit-form-dialog'

interface OrganizationRateLimitDrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  orgType: string
  orgId: number
  orgName: string
}

const WEEKDAY_NAMES = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']

export function OrganizationRateLimitDrawer({
  open,
  onOpenChange,
  orgType,
  orgId,
  orgName,
}: OrganizationRateLimitDrawerProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [formOpen, setFormOpen] = useState(false)
  const [editingRule, setEditingRule] = useState<RateLimitRule | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ['rate-limits', orgType, orgId],
    queryFn: async () => {
      const result = await getRateLimits({
        org_type: orgType,
        org_id: orgId,
        page_size: 100,
      })
      if (!result.success) {
        toast.error(result.message || 'Failed to load rules')
        return []
      }
      return (result.data?.items || []) as RateLimitRule[]
    },
    enabled: open,
  })

  const rules = data || []

  const handleDelete = async (rule: RateLimitRule) => {
    try {
      const res = await deleteRateLimit(rule.id)
      if (res.success) {
        toast.success(t('Rule deleted successfully'))
        queryClient.invalidateQueries({ queryKey: ['rate-limits', orgType, orgId] })
      } else {
        toast.error(res.message || t('Failed to delete rule'))
      }
    } catch {
      toast.error(t('Failed to delete rule'))
    }
  }

  const handleFormSuccess = () => {
    setFormOpen(false)
    setEditingRule(null)
    queryClient.invalidateQueries({ queryKey: ['rate-limits', orgType, orgId] })
  }

  return (
    <>
      <Sheet open={open} onOpenChange={onOpenChange}>
        <SheetContent className='sm:max-w-lg'>
          <SheetHeader>
            <SheetTitle>{t('Organization RPM Rules')}</SheetTitle>
            <SheetDescription>
              {orgName} ({t(orgType === 'company' ? 'Company' : 'Department')})
              {' - '}
              {t('Total rules')}: {rules.length}
            </SheetDescription>
          </SheetHeader>
          <div className='px-4 pb-2'>
            <Button
              size='sm'
              onClick={() => {
                setEditingRule(null)
                setFormOpen(true)
              }}
            >
              <Plus />
              {t('Add RPM Rule')}
            </Button>
          </div>
          <ScrollArea className='flex-1 px-4'>
            {isLoading ? (
              <div className='text-muted-foreground text-sm'>
                {t('Loading...')}
              </div>
            ) : rules.length === 0 ? (
              <div className='text-muted-foreground text-sm'>
                {t('No rules configured')}
              </div>
            ) : (
              <div className='space-y-3 pb-4'>
                {rules.map((rule) => (
                  <RuleCard
                    key={rule.id}
                    rule={rule}
                    onEdit={() => {
                      setEditingRule(rule)
                      setFormOpen(true)
                    }}
                    onDelete={() => handleDelete(rule)}
                  />
                ))}
              </div>
            )}
          </ScrollArea>
        </SheetContent>
      </Sheet>

      <OrganizationRateLimitFormDialog
        open={formOpen}
        onOpenChange={setFormOpen}
        orgType={orgType}
        orgId={orgId}
        editingRule={editingRule}
        onSuccess={handleFormSuccess}
      />
    </>
  )
}

function RuleCard({
  rule,
  onEdit,
  onDelete,
}: {
  rule: RateLimitRule
  onEdit: () => void
  onDelete: () => void
}) {
  const { t } = useTranslation()

  return (
    <div className='rounded-lg border p-3 space-y-2'>
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-2'>
          <span className='font-medium text-sm'>
            {rule.model_name || t('All Models')}
          </span>
          <Badge variant={rule.status === 1 ? 'default' : 'destructive'} className='text-xs'>
            {rule.status === 1 ? t('Enabled') : t('Disabled')}
          </Badge>
        </div>
        <div className='flex items-center gap-1'>
          <Button variant='ghost' size='icon-sm' onClick={onEdit}>
            <Pencil className='h-3.5 w-3.5' />
          </Button>
          <Button variant='ghost' size='icon-sm' onClick={onDelete}>
            <Trash2 className='h-3.5 w-3.5 text-destructive' />
          </Button>
        </div>
      </div>
      <div className='text-xs text-muted-foreground'>
        {t('Priority')}: {rule.priority}
      </div>
      <div className='space-y-1'>
        {rule.time_slots.map((slot, i) => (
          <div key={i} className='flex items-center gap-2 text-xs'>
            <span className='font-mono'>
              {slot.start_time}-{slot.end_time}
            </span>
            <span>
              {slot.weekdays && slot.weekdays.length > 0
                ? slot.weekdays.map((w) => t(WEEKDAY_NAMES[w])).join(', ')
                : t('Every day')}
            </span>
            <Badge variant='outline' className='text-xs'>
              {rule.rpms[i] || 0} RPM
            </Badge>
          </div>
        ))}
      </div>
    </div>
  )
}
