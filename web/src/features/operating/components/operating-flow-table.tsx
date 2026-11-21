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
import type { PaginationState } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'

import { useOperatingFlow } from '../hooks/use-operating'
import type { OperatingRange } from '../types'

import { useOperatingFlowColumns } from './operating-flow-columns'

const FLOW_PAGE_SIZE = 20

interface OperatingFlowTableProps {
  range: OperatingRange
}

/**
 * 流水日志：收入、支出与发放按发生时间倒序，分页与通用日志一致。
 */
export function OperatingFlowTable(props: OperatingFlowTableProps) {
  const { t } = useTranslation()
  const columns = useOperatingFlowColumns()
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: FLOW_PAGE_SIZE,
  })

  const flowQuery = useOperatingFlow(
    props.range,
    pagination.pageIndex + 1,
    pagination.pageSize
  )
  const total = flowQuery.data?.total ?? 0

  const { table } = useDataTable({
    data: flowQuery.data?.items ?? [],
    columns,
    enableRowSelection: false,
    enableSorting: false,
    manualPagination: true,
    manualFiltering: true,
    totalCount: total,
    pagination,
    onPaginationChange: setPagination,
    initialPagination: { pageIndex: 0, pageSize: FLOW_PAGE_SIZE },
  })

  const rangeLabel = useMemo(
    () =>
      t('{{count}} records', {
        count: total,
      }),
    [total, t]
  )

  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={flowQuery.isLoading}
        isFetching={flowQuery.isFetching}
        emptyTitle={t('No flow records in this range')}
        emptyDescription={t(
          'Income, expenses and issued credits appear here once they happen in the selected range.'
        )}
        toolbarProps={null}
        hideMobile
      />
      <p className='text-muted-foreground text-xs'>{rangeLabel}</p>
    </div>
  )
}
