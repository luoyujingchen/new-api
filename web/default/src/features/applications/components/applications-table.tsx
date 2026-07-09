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
import { useState } from 'react'
import {
  flexRender,
  getCoreRowModel,
  getPaginationRowModel,
  useReactTable,
  type ColumnDef,
  type PaginationState,
} from '@tanstack/react-table'
import { MoreHorizontal, Pencil, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { DataTablePagination } from '@/components/data-table/pagination'
import { deleteApplication, updateApplicationStatus } from '../api'
import { type Application } from '../types'

interface ApplicationsTableProps {
  data: Application[]
  total: number
  page: number
  pageSize: number
  onPageChange: (page: number) => void
  onEdit: (application: Application) => void
  onRefresh: () => void
}

export function ApplicationsTable({
  data,
  total,
  page,
  pageSize,
  onPageChange,
  onEdit,
  onRefresh,
}: ApplicationsTableProps) {
  const { t } = useTranslation()

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: page - 1,
    pageSize,
  })

  const columns = useColumns({
    onEdit,
    onRefresh,
  })

  const table = useReactTable({
    data,
    columns,
    pageCount: Math.ceil(total / pageSize),
    state: { pagination },
    onPaginationChange: (updater) => {
      const newPagination =
        typeof updater === 'function' ? updater(pagination) : updater
      setPagination(newPagination)
      if (newPagination.pageIndex !== page - 1) {
        onPageChange(newPagination.pageIndex + 1)
      }
    },
    getCoreRowModel: getCoreRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  })

  return (
    <div className='space-y-4'>
      <div className='px-2'>
        <span className='text-muted-foreground text-sm'>
          {t('Total {{count}} items', { count: total })}
        </span>
      </div>

      <div className='rounded-md border'>
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id} colSpan={header.colSpan}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(
                          header.column.columnDef.header,
                          header.getContext()
                        )}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {table.getRowModel().rows?.length ? (
              table.getRowModel().rows.map((row) => (
                <TableRow
                  key={row.id}
                  data-state={row.getIsSelected() && 'selected'}
                >
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(
                        cell.column.columnDef.cell,
                        cell.getContext()
                      )}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : (
              <TableRow>
                <TableCell
                  colSpan={columns.length}
                  className='h-24 text-center'
                >
                  {t('No results')}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </div>

      <DataTablePagination table={table} />
    </div>
  )
}

function useColumns({
  onEdit,
  onRefresh,
}: {
  onEdit: (application: Application) => void
  onRefresh: () => void
}) {
  const { t } = useTranslation()

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(t('Are you sure you want to delete "{{name}}"?', { name }))) {
      return
    }

    const result = await deleteApplication(id)
    if (result.success) {
      toast.success(t('Application deleted successfully'))
      onRefresh()
    } else {
      toast.error(result.message || t('Failed to delete application'))
    }
  }

  const handleToggleStatus = async (id: number, currentStatus: number) => {
    const newStatus = currentStatus === 1 ? 0 : 1

    const result = await updateApplicationStatus(id, newStatus)
    if (result.success) {
      toast.success(
        newStatus === 1
          ? t('Application enabled successfully')
          : t('Application disabled successfully')
      )
      onRefresh()
    } else {
      toast.error(result.message || t('Failed to update application status'))
    }
  }

  const formatHeaderRules = (application: Application) => {
    const rules = application.header_validation_rules || []
    if (rules.length === 0) {
      return t('No header rules')
    }
    const preview = rules
      .slice(0, 2)
      .map((rule) => {
        const value =
          rule.operator === 'one_of'
            ? `[${(rule.values || []).join(', ')}]`
            : rule.value
        return `${rule.header} ${rule.operator} ${value || ''}`
      })
      .join(', ')
    return rules.length > 2
      ? t('{{count}} rules: {{preview}}...', {
          count: rules.length,
          preview,
        })
      : t('{{count}} rules: {{preview}}', {
          count: rules.length,
          preview,
        })
  }

  const columns: ColumnDef<Application>[] = [
    {
      accessorKey: 'name',
      header: t('Application Name'),
      cell: ({ row }) => (
        <span className='font-medium'>{row.getValue('name')}</span>
      ),
    },
    {
      accessorKey: 'app_key',
      header: t('App Key'),
      cell: ({ row }) => {
        const key = row.getValue('app_key') as string
        return (
          <span className='text-muted-foreground font-mono text-sm'>{key}</span>
        )
      },
    },
    {
      accessorKey: 'description',
      header: t('Description'),
      cell: ({ row }) => {
        const desc = row.getValue('description') as string
        return desc ? (
          <span className='text-muted-foreground block max-w-[200px] truncate text-sm'>
            {desc}
          </span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        )
      },
    },
    {
      accessorKey: 'token_count',
      header: t('API Keys'),
      cell: ({ row }) => (
        <span>{(row.getValue('token_count') as number) ?? 0}</span>
      ),
    },
    {
      id: 'header_validation_rules',
      header: t('Header Rules'),
      cell: ({ row }) => {
        return (
          <span className='text-muted-foreground block max-w-[280px] truncate text-sm'>
            {formatHeaderRules(row.original)}
          </span>
        )
      },
    },
    {
      accessorKey: 'header_match_required',
      header: t('Header Strict Match'),
      cell: ({ row }) => {
        const required = row.getValue('header_match_required') as boolean
        return (
          <Badge variant={required ? 'default' : 'secondary'}>
            {required ? t('Required') : t('Observe only')}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as number
        return (
          <Badge
            variant='secondary'
            className={
              status === 1
                ? 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300'
                : undefined
            }
          >
            {status === 1 ? t('Enabled') : t('Disabled')}
          </Badge>
        )
      },
    },
    {
      id: 'actions',
      header: t('Actions'),
      cell: ({ row }) => {
        const application = row.original
        return (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant='ghost' size='icon' />}
            >
              <MoreHorizontal className='h-4 w-4' />
            </DropdownMenuTrigger>
            <DropdownMenuContent align='end'>
              <DropdownMenuGroup>
                <DropdownMenuItem
                  onClick={() =>
                    handleToggleStatus(application.id, application.status)
                  }
                >
                  {application.status === 1 ? t('Disable') : t('Enable')}
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => onEdit(application)}>
                  <Pencil className='mr-2 h-4 w-4' />
                  {t('Edit')}
                </DropdownMenuItem>
                <DropdownMenuItem
                  className='text-destructive'
                  onClick={() => handleDelete(application.id, application.name)}
                >
                  <Trash2 className='mr-2 h-4 w-4' />
                  {t('Delete')}
                </DropdownMenuItem>
              </DropdownMenuGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]

  return columns
}
