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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { formatLocalCurrencyAmount } from '@/lib/currency'
import { formatQuota, formatTimestamp } from '@/lib/format'

import {
  OPERATING_EXPENSE_CATEGORY_LABEL_KEYS,
  OPERATING_GRANT_LABEL_KEYS,
} from '../constants'
import type { OperatingFlowEvent } from '../types'

/** 流水来源展示名：收入是支付渠道，支出是类别，发放是发放来源。 */
function sourceLabelKey(event: OperatingFlowEvent): string {
  if (event.kind === 'expense') {
    return OPERATING_EXPENSE_CATEGORY_LABEL_KEYS[event.source] ?? event.source
  }
  if (event.kind === 'grant') {
    return OPERATING_GRANT_LABEL_KEYS[event.source] ?? event.source
  }
  return event.source
}

export function useOperatingFlowColumns(): ColumnDef<OperatingFlowEvent>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'occurred_at',
      header: () => t('Time'),
      cell: ({ row }) => (
        <span className='whitespace-nowrap tabular-nums'>
          {formatTimestamp(row.original.occurred_at)}
        </span>
      ),
    },
    {
      accessorKey: 'kind',
      header: () => t('Type'),
      cell: ({ row }) => {
        if (row.original.kind === 'income') {
          return <Badge variant='outline'>{t('Income')}</Badge>
        }
        if (row.original.kind === 'expense') {
          return <Badge variant='destructive'>{t('Expense')}</Badge>
        }
        return <Badge variant='secondary'>{t('Credits')}</Badge>
      },
    },
    {
      accessorKey: 'source',
      header: () => t('Source'),
      cell: ({ row }) => (
        <span className='whitespace-nowrap'>{t(sourceLabelKey(row.original))}</span>
      ),
    },
    {
      accessorKey: 'username',
      header: () => t('User'),
      cell: ({ row }) => row.original.username || '-',
    },
    {
      id: 'amount',
      header: () => t('Amount'),
      cell: ({ row }) => (
        <span className='whitespace-nowrap tabular-nums'>
          {row.original.kind === 'grant'
            ? formatQuota(row.original.quota)
            : formatLocalCurrencyAmount(row.original.amount_cents / 100)}
          {row.original.accuracy === 'estimated' ? (
            <span className='text-muted-foreground/60 ml-1 text-xs'>?</span>
          ) : null}
        </span>
      ),
    },
    {
      accessorKey: 'note',
      header: () => t('Note'),
      cell: ({ row }) => (
        <span className='text-muted-foreground block max-w-80 truncate'>
          {row.original.note || '-'}
        </span>
      ),
    },
  ]
}
