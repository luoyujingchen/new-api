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
import { useTranslation } from 'react-i18next'
import { MoreHorizontal, Pencil, Trash2, Building2, Gauge } from 'lucide-react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
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
import { Badge } from '@/components/ui/badge'
import { deleteDepartment, updateDepartmentStatus } from '../api'
import { type Department, DEPARTMENT_LEVEL_LABELS } from '../types'

interface DepartmentsTableProps {
  data: Department[]
  total: number
  page: number
  pageSize: number
  onPageChange: (page: number) => void
  onEdit: (department: Department) => void
  onConfigureRateLimit: (department: Department) => void
  onRefresh: () => void
}

export function DepartmentsTable({
  data,
  total,
  page,
  pageSize,
  onPageChange,
  onEdit,
  onConfigureRateLimit,
  onRefresh,
}: DepartmentsTableProps) {
  const { t } = useTranslation()

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: page - 1,
    pageSize,
  })

  const columns = useColumns({ onEdit, onConfigureRateLimit, onRefresh })

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
    <div className="space-y-4">
      <div className="px-2">
        <span className="text-sm text-muted-foreground">
          {t('Total {{count}} items', { count: total })}
        </span>
      </div>

      <div className="rounded-md border">
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
                  className="h-24 text-center"
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
  onConfigureRateLimit,
  onRefresh,
}: {
  onEdit: (department: Department) => void
  onConfigureRateLimit: (department: Department) => void
  onRefresh: () => void
}) {
  const { t } = useTranslation()

  const handleDelete = async (id: number, name: string) => {
    if (!confirm(t('Are you sure you want to delete "{{name}}"?', { name }))) {
      return
    }

    const result = await deleteDepartment(id)
    if (result.success) {
      toast.success(t('Department deleted successfully'))
      onRefresh()
    } else {
      toast.error(result.message || t('Failed to delete department'))
    }
  }

  const handleToggleStatus = async (id: number, currentStatus: number, name: string) => {
    const newStatus = currentStatus === 1 ? 0 : 1
    const action = newStatus === 1 ? t('enable') : t('disable')

    const result = await updateDepartmentStatus(id, newStatus)
    if (result.success) {
      toast.success(
        t('Department "{{name}}" {{action}}d successfully', { name, action })
      )
      onRefresh()
    } else {
      toast.error(result.message || t('Failed to update department status'))
    }
  }

  const columns: ColumnDef<Department>[] = [
    {
      accessorKey: 'name',
      header: t('Department Name'),
      cell: ({ row }) => {
        const level = row.original.level
        const indent = (level - 1) * 24
        return (
          <div
            className="flex items-center"
            style={{ paddingLeft: `${indent}px` }}
          >
            <span className="font-medium">{row.getValue('name')}</span>
          </div>
        )
      },
    },
    {
      accessorKey: 'level',
      header: t('Level'),
      cell: ({ row }) => {
        const level = row.getValue('level') as number
        return (
          <Badge variant="outline">
            {DEPARTMENT_LEVEL_LABELS[level]?.zh || `${level}级`}
          </Badge>
        )
      },
    },
    {
      accessorKey: 'company',
      header: t('Company'),
      cell: ({ row }) => {
        const company = row.original.company
        return company ? (
          <div className="flex items-center gap-1">
            <Building2 className="h-3 w-3 text-muted-foreground" />
            <span className="text-sm">{company.name}</span>
          </div>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      accessorKey: 'description',
      header: t('Description'),
      cell: ({ row }) => {
        const desc = row.getValue('description') as string
        return desc ? (
          <span className="text-sm text-muted-foreground truncate max-w-[200px] block">
            {desc}
          </span>
        ) : (
          <span className="text-muted-foreground">-</span>
        )
      },
    },
    {
      accessorKey: 'user_count',
      header: t('Users'),
      cell: ({ row }) => {
        const count = row.getValue('user_count') as number
        return <span>{count ?? 0}</span>
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
        const department = row.original
        return (
          <DropdownMenu>
            <DropdownMenuTrigger
              render={<Button variant='ghost' size='icon' />}
            >
              <MoreHorizontal className='h-4 w-4' />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => onConfigureRateLimit(department)}>
                <Gauge className='mr-2 h-4 w-4' />
                {t('Configure RPM')}
              </DropdownMenuItem>
              <DropdownMenuItem
                onClick={() => handleToggleStatus(department.id, department.status, department.name)}
              >
                {department.status === 1 ? t('Disable') : t('Enable')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onEdit(department)}>
                <Pencil className="mr-2 h-4 w-4" />
                {t('Edit')}
              </DropdownMenuItem>
              <DropdownMenuItem
                className="text-destructive"
                onClick={() => handleDelete(department.id, department.name)}
              >
                <Trash2 className="mr-2 h-4 w-4" />
                {t('Delete')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )
      },
    },
  ]

  return columns
}
