import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
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
import { getCompanies } from '../../api'
import { useCompaniesColumns } from './companies-columns'
import { useCompanies } from './companies-provider'

const route = getRouteApi('/_authenticated/organizations/companies/')

export function CompaniesTable() {
  const { t } = useTranslation()
  const columns = useCompaniesColumns()
  const { refreshTrigger } = useCompanies()
  const [rowSelection, setRowSelection] = useState({})
  const [sorting, setSorting] = useState<SortingState>([])
  const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({})

  const search = route.useSearch()
  const page = search.page ?? 1
  const pageSize = search.pageSize ?? 10

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['companies', page, pageSize, refreshTrigger],
    queryFn: async () => {
      const result = await getCompanies({ p: page, page_size: pageSize })
      if (!result.success) {
        toast.error(result.message || 'Failed to load companies')
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items || [],
        total: result.data?.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const companies = data?.items || []
  const pagination = {
    pageIndex: page - 1,
    pageSize,
  }

  const table = useReactTable({
    data: companies,
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

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Companies Found')}
      emptyDescription={t(
        'No companies available. Create your first company.'
      )}
      skeletonKeyPrefix='companies-skeleton'
    />
  )
}
