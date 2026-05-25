import { useMemo, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { LearnMore } from '@/components/learn-more'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { getQueueModelStatus, getQueueStatus } from '../api'
import type { QueueModelSnapshot } from '../types'

type QueueMonitorRow = QueueModelSnapshot & {
  model_name: string
}

function formatWait(value: number, unit: string) {
  const normalized = value >= 10 ? value.toFixed(0) : value.toFixed(1)
  return `${normalized} ${unit}`
}

function formatBuckets(buckets: Record<string, number>) {
  return Object.entries(buckets).sort((left, right) => {
    return Number(right[0]) - Number(left[0])
  })
}

function QueueMetricHelp({
  label,
  description,
}: {
  label: string
  description: string
}) {
  return (
    <div className="flex items-center gap-1 whitespace-nowrap">
      <span>{label}</span>
      <LearnMore
        contentProps={{
          align: 'center',
          className: 'max-w-80 leading-relaxed',
        }}
        triggerProps={{
          className:
            'size-4 shrink-0 border-none bg-transparent p-0 text-muted-foreground shadow-none hover:bg-transparent hover:text-foreground',
        }}
      >
        <p>{description}</p>
      </LearnMore>
    </div>
  )
}

export function QueueMonitorPage() {
  const { t } = useTranslation()
  const [selectedModel, setSelectedModel] = useState<string | null>(null)

  const queueStatusQuery = useQuery({
    queryKey: ['queue-status'],
    queryFn: getQueueStatus,
    refetchInterval: 5000,
  })

  const queueModelQuery = useQuery({
    queryKey: ['queue-status', selectedModel],
    queryFn: () => getQueueModelStatus(selectedModel || ''),
    enabled: !!selectedModel,
    refetchInterval: selectedModel ? 5000 : false,
  })

  const rows = useMemo<QueueMonitorRow[]>(() => {
    return Object.entries(queueStatusQuery.data?.models || {})
      .map(([model_name, snapshot]) => ({ model_name, ...snapshot }))
      .sort((left, right) => {
        if (right.queued !== left.queued) {
          return right.queued - left.queued
        }
        return left.model_name.localeCompare(right.model_name)
      })
  }, [queueStatusQuery.data])

  const queuedModels = rows.filter((row) => row.queued > 0).length
  const columnHelp = {
    throughput: t(
      'Requests dequeued from this model queue during the last 60 seconds. This is queue throughput, not upstream provider RPM.'
    ),
    priorityBuckets: t(
      'Current queued request counts grouped by effective priority buckets P1 to P10. Higher priorities get stronger scheduling weight.'
    ),
  }

  if (queueStatusQuery.isLoading) {
    return <div className="p-4">{t('Loading...')}</div>
  }

  if (queueStatusQuery.isError) {
    return <div className="p-4">{t('Failed to load queue status')}</div>
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t('Queue Monitor')}</h1>
          <p className="text-sm text-muted-foreground">
            {t('Monitor per-model queue depth, wait times, and weighted priority buckets.')}
          </p>
        </div>
        <Button
          variant="outline"
          onClick={() => queueStatusQuery.refetch()}
          disabled={queueStatusQuery.isFetching}
        >
          <RefreshCw className="mr-2 h-4 w-4" />
          {t('Refresh')}
        </Button>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardDescription>{t('Queue status')}</CardDescription>
            <CardTitle>
              {queueStatusQuery.data?.queue_enabled ? t('Enabled') : t('Disabled')}
            </CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t('Total queued requests')}</CardDescription>
            <CardTitle>{queueStatusQuery.data?.total_queued ?? 0}</CardTitle>
          </CardHeader>
        </Card>
        <Card>
          <CardHeader>
            <CardDescription>{t('Models with queued requests')}</CardDescription>
            <CardTitle>{queuedModels}</CardTitle>
          </CardHeader>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('Per-model queues')}</CardTitle>
          <CardDescription>
            {t('The monitor refreshes automatically every 5 seconds.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="rounded-md border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Model')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Queued')}</TableHead>
                  <TableHead>{t('Average wait')}</TableHead>
                  <TableHead>{t('Max wait')}</TableHead>
                  <TableHead>
                    <QueueMetricHelp
                      label={t('Throughput')}
                      description={columnHelp.throughput}
                    />
                  </TableHead>
                  <TableHead>{t('Queue limit')}</TableHead>
                  <TableHead>
                    <QueueMetricHelp
                      label={t('Priority buckets')}
                      description={columnHelp.priorityBuckets}
                    />
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.length > 0 ? (
                  rows.map((row) => (
                    <TableRow key={row.model_name}>
                      <TableCell>
                        <button
                          type="button"
                          className="text-left font-medium text-foreground transition-colors hover:text-primary hover:underline"
                          onClick={() => setSelectedModel(row.model_name)}
                          title={t('Click to view full details')}
                        >
                          {row.model_name}
                        </button>
                      </TableCell>
                      <TableCell>
                        <Badge variant="secondary">
                          {row.enabled ? t('Enabled') : t('Disabled')}
                        </Badge>
                      </TableCell>
                      <TableCell>{row.queued}</TableCell>
                      <TableCell>{formatWait(row.avg_wait_sec, t('seconds'))}</TableCell>
                      <TableCell>{formatWait(row.max_wait_sec, t('seconds'))}</TableCell>
                      <TableCell>{row.throughput_rpm}</TableCell>
                      <TableCell>
                        {row.max_queue_size === 0 ? t('Unlimited') : row.max_queue_size}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {formatBuckets(row.buckets)
                            .filter(([, count]) => count > 0)
                            .map(([priority, count]) => (
                              <Badge key={priority} variant="outline">
                                {`P${priority}: ${count}`}
                              </Badge>
                            ))}
                          {Object.values(row.buckets).every((count) => count === 0) && (
                            <span className="text-sm text-muted-foreground">
                              {t('No queued requests')}
                            </span>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                ) : (
                  <TableRow>
                    <TableCell colSpan={8} className="h-24 text-center">
                      {t('No queue activity yet')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Sheet open={!!selectedModel} onOpenChange={(open) => !open && setSelectedModel(null)}>
        <SheetContent className="sm:max-w-xl">
          <SheetHeader>
            <SheetTitle>{selectedModel || t('Queue details')}</SheetTitle>
            <SheetDescription>
              {t('Detailed live metrics for a single model queue.')}
            </SheetDescription>
          </SheetHeader>
          <div className="mt-6 space-y-4">
            {queueModelQuery.isLoading || queueModelQuery.isFetching ? (
              <div className="text-sm text-muted-foreground">{t('Loading...')}</div>
            ) : queueModelQuery.data ? (
              <>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Card>
                    <CardHeader>
                      <CardDescription>{t('Queued')}</CardDescription>
                      <CardTitle>{queueModelQuery.data.queued}</CardTitle>
                    </CardHeader>
                  </Card>
                  <Card>
                    <CardHeader>
                      <CardDescription>{t('Throughput')}</CardDescription>
                      <CardTitle>{queueModelQuery.data.throughput_rpm}</CardTitle>
                    </CardHeader>
                  </Card>
                  <Card>
                    <CardHeader>
                      <CardDescription>{t('Average wait')}</CardDescription>
                      <CardTitle>
                        {formatWait(queueModelQuery.data.avg_wait_sec, t('seconds'))}
                      </CardTitle>
                    </CardHeader>
                  </Card>
                  <Card>
                    <CardHeader>
                      <CardDescription>{t('Max wait')}</CardDescription>
                      <CardTitle>
                        {formatWait(queueModelQuery.data.max_wait_sec, t('seconds'))}
                      </CardTitle>
                    </CardHeader>
                  </Card>
                </div>

                <Card>
                  <CardHeader>
                    <CardTitle>{t('Priority buckets')}</CardTitle>
                    <CardDescription>
                      {t('Queued requests grouped by effective priority from 1 to 10.')}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-wrap gap-2">
                      {formatBuckets(queueModelQuery.data.buckets).map(
                        ([priority, count]) => (
                          <Badge key={priority} variant="outline">
                            {`P${priority}: ${count}`}
                          </Badge>
                        )
                      )}
                    </div>
                  </CardContent>
                </Card>
              </>
            ) : (
              <div className="text-sm text-muted-foreground">
                {t('Failed to load queue model details')}
              </div>
            )}
          </div>
        </SheetContent>
      </Sheet>
    </div>
  )
}
