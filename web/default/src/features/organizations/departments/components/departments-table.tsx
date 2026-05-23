import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useSearch } from '@tanstack/react-router'
import {
  type SortingState,
  type VisibilityState,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  getSortedRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { DataTablePage } from '@/components/data-table'
import { getDepartments, getCompany } from '../../api'
import { useDepartmentsColumns } from './departments-columns'
import { useDepartments } from './departments-provider'

export function DepartmentsTable() {
  const { t } = useTranslation()
  const columns = useDepartmentsColumns()
  const { refreshTrigger } = useDepartments()
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const search = useSearch({ strict: false }) as {
    page?: number
    pageSize?: number
    company_id?: number
  }
  const page = search.page ?? 1
  const pageSize = search.pageSize ?? 10
  const companyId = search.company_id

  const { data: companyData } = useQuery({
    queryKey: ['company', companyId],
    queryFn: async () => {
      if (!companyId) return null
      const res = await getCompany(companyId)
      return res.data || null
    },
    enabled: !!companyId,
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'departments',
      page,
      pageSize,
      companyId,
      refreshTrigger,
    ],
    queryFn: async () => {
      const params: { p: number; page_size: number; company_id?: number } = {
        p: page,
        page_size: pageSize,
      }
      if (companyId) params.company_id = companyId
      const result = await getDepartments(params)
      if (!result.success) {
        toast.error(result.message || 'Failed to load departments')
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const departments = data?.items || []
  const pagination = {
    pageIndex: page - 1,
    pageSize,
  }

  const table = useReactTable({
    data: departments,
    columns,
    state: {
      sorting,
      columnVisibility,
      rowSelection,
      pagination,
    },
    enableRowSelection: true,
    onRowSelectionChange: setRowSelection,
    onSortingChange: setSorting,
    onColumnVisibilityChange: setColumnVisibility,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualPagination: true,
    pageCount: Math.ceil((data?.total || 0) / pageSize),
  })

  const titleSuffix = companyData
    ? ` - ${companyData.name}`
    : ''

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Departments Found')}
      emptyDescription={t(
        'No departments available. Create your first department.'
      )}
      skeletonKeyPrefix='departments-skeleton'
    />
  )
}
