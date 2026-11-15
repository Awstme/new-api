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
import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { TableCell, TableRow } from '@/components/ui/table'
import {
  formatCompactNumber,
  formatLogQuota,
  formatTimestampToDate,
  formatUseTime,
} from '@/lib/format'

import { expandLogs } from '../api'
import type { UsageLog } from '../data/schema'
import { parseLogOther } from '../lib/format'
import { getLogTypeConfig } from '../lib/utils'

function RawLogsTable(props: { rows: UsageLog[]; showRequestId: boolean }) {
  const { t } = useTranslation()
  return (
    <div className='max-h-[360px] overflow-auto rounded-md border bg-background'>
      <table className='w-full min-w-[840px] text-[11px]'>
        <thead className='bg-background sticky top-0 z-10'>
          <tr className='text-muted-foreground border-b text-left'>
            {props.showRequestId && (
              <th className='px-2 py-1.5 font-medium'>{t('Request ID')}</th>
            )}
            <th className='px-2 py-1.5 font-medium'>{t('Time')}</th>
            <th className='px-2 py-1.5 font-medium'>{t('Type')}</th>
            <th className='px-2 py-1.5 font-medium'>{t('Channel')}</th>
            <th className='px-2 py-1.5 font-medium'>{t('Status')}</th>
            <th className='px-2 py-1.5 text-right font-medium'>{t('Token')}</th>
            <th className='px-2 py-1.5 text-right font-medium'>{t('Cost')}</th>
            <th className='px-2 py-1.5 text-right font-medium'>
              {t('Timing')}
            </th>
            <th className='px-2 py-1.5 font-medium'>{t('Content')}</th>
          </tr>
        </thead>
        <tbody>
          {props.rows.map((row) => {
            const other = parseLogOther(row.other)
            const config = getLogTypeConfig(row.type)
            const isError = row.type === 5
            const content = isError
              ? row.content.replace(/^status_code=\d+,\s*/, '')
              : ''
            return (
              <tr key={row.id} className='border-border/50 border-t'>
                {props.showRequestId && (
                  <td className='text-muted-foreground max-w-[180px] truncate px-2 py-1.5 font-mono text-[10px]'>
                    {row.request_id}
                  </td>
                )}
                <td className='px-2 py-1.5 font-mono text-[10px] whitespace-nowrap'>
                  {formatTimestampToDate(row.created_at)}
                </td>
                <td className='px-2 py-1.5 whitespace-nowrap'>
                  <span
                    className={
                      isError
                        ? 'font-medium text-red-600 dark:text-red-400'
                        : 'font-medium text-emerald-600 dark:text-emerald-400'
                    }
                  >
                    {t(config.label)}
                  </span>
                </td>
                <td className='px-2 py-1.5 font-mono text-[10px] whitespace-nowrap'>
                  #{row.channel}
                </td>
                <td className='px-2 py-1.5 whitespace-nowrap'>
                  {other?.status_code ? (
                    <span className='font-medium text-red-600 dark:text-red-400'>
                      HTTP {other.status_code}
                    </span>
                  ) : (
                    <span className='text-muted-foreground'>-</span>
                  )}
                </td>
                <td
                  className='px-2 py-1.5 text-right font-mono text-[10px] whitespace-nowrap'
                  title={`${row.prompt_tokens.toLocaleString()} / ${row.completion_tokens.toLocaleString()}`}
                >
                  {formatCompactNumber(row.prompt_tokens)} /{' '}
                  {formatCompactNumber(row.completion_tokens)}
                </td>
                <td className='px-2 py-1.5 text-right whitespace-nowrap'>
                  {formatLogQuota(row.quota)}
                </td>
                <td className='px-2 py-1.5 text-right whitespace-nowrap'>
                  {formatUseTime(row.use_time)}
                </td>
                <td
                  className='text-muted-foreground max-w-[260px] truncate px-2 py-1.5 text-[10px]'
                  title={content}
                >
                  {content || '-'}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

interface ExpandedLogRowsProps {
  log: UsageLog
  colSpan: number
}

export function ExpandedLogRows(props: ExpandedLogRowsProps) {
  const { t } = useTranslation()
  const other = parseLogOther(props.log.other)
  const requestIds = useMemo(() => {
    const fromSession = Array.isArray(other?.session_requests)
      ? other.session_requests
          .map((request) => request.request_id)
          .filter((id) => id && !id.startsWith('legacy:'))
      : []
    let ids: string[] = []
    if (fromSession.length > 0) {
      ids = fromSession
    } else if (
      props.log.request_id &&
      !props.log.request_id.startsWith('legacy:')
    ) {
      ids = [props.log.request_id]
    }
    return [...new Set(ids)]
  }, [other?.session_requests, props.log.request_id])
  const query = useQuery({
    queryKey: ['log-expand', requestIds.join(',')],
    queryFn: () => expandLogs(requestIds),
    enabled: requestIds.length > 0,
    staleTime: 30 * 1000,
    retry: false,
  })
  const rows = query.data?.data?.items ?? []
  const truncated = query.data?.data?.truncated === true
  const showRequestId = requestIds.length > 1

  let body: ReactNode
  if (requestIds.length === 0) {
    body = (
      <p className='text-muted-foreground text-xs'>
        {t('Raw logs are unavailable for this request.')}
      </p>
    )
  } else if (query.isLoading) {
    body = (
      <div className='text-muted-foreground flex items-center gap-2 py-3 text-xs'>
        <Loader2 className='size-3.5 animate-spin' />
        {t('Loading raw logs')}
      </div>
    )
  } else if (query.isError) {
    body = (
      <p className='text-destructive text-xs'>{t('Failed to load raw logs')}</p>
    )
  } else {
    body = <RawLogsTable rows={rows} showRequestId={showRequestId} />
  }

  return (
    <TableRow className='bg-muted/20 hover:bg-muted/20'>
      <TableCell colSpan={props.colSpan} className='p-0'>
        <div className='border-border/60 border-t px-3 py-2.5 sm:px-4'>
          <div className='mb-2 flex flex-wrap items-center gap-x-2 gap-y-1'>
            <span className='text-xs font-semibold'>{t('Raw log rows')}</span>
            <span className='text-muted-foreground text-xs'>
              {rows.length} {t('log rows')}
            </span>
            {truncated && (
              <span className='text-muted-foreground text-xs'>
                {t('Only the first 500 rows are shown.')}
              </span>
            )}
          </div>
          {body}
        </div>
      </TableCell>
    </TableRow>
  )
}