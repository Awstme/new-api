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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import {
  formatCompactNumber,
  formatLogQuota,
  formatPercent,
  formatUseTime,
} from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import type { LogStatisticsBucket } from '../types'
import { useLogsViewScope, useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatBadge(props: {
  label: string
  value: string | number
  accent: string
  title?: string
}) {
  return (
    <span
      className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'
      title={props.title}
    >
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

function StatSparkline(props: { series?: LogStatisticsBucket[] }) {
  const series = props.series ?? []
  if (series.length < 2) return null
  const values = series.map((item) => item.requests)
  const maxValue = Math.max(1, ...values)
  const width = 96
  const height = 24
  const step = width / Math.max(1, values.length - 1)
  const points = values.map((value, index) => {
    const x = index * step
    const y = height - (value / maxValue) * (height - 3) - 1.5
    return `${x.toFixed(1)},${y.toFixed(1)}`
  })
  const line = `M${points.join(' L')}`
  const area = `${line} L${width},${height} L0,${height} Z`
  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className='h-6 w-24 shrink-0 text-sky-500/70'
      aria-hidden='true'
    >
      <path d={area} fill='currentColor' opacity='0.12' />
      <path
        d={line}
        fill='none'
        stroke='currentColor'
        strokeWidth='1.4'
        strokeLinejoin='round'
        strokeLinecap='round'
      />
    </svg>
  )
}

export function CommonLogsStats() {
  const { t } = useTranslation()
  const { isAdminView: isAdmin } = useLogsViewScope()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const { data: stats, isLoading } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? requireServerSuccess(await getLogStats(params))
        : requireServerSuccess(await getUserLogStats(params))

      return result.success
        ? result.data || DEFAULT_LOG_STATS
        : DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
  })

  if (isLoading) {
    return (
      <div className='flex items-center gap-2'>
        <Skeleton className='h-7 w-[150px] rounded-md' />
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
      </div>
    )
  }

  const requests = stats?.requests ?? 0
  const completionRate =
    requests > 0 ? ((stats?.success_requests ?? 0) / requests) * 100 : 0
  const failed = stats?.failed_requests ?? 0
  const retried = stats?.retried_requests ?? 0
  const quota = stats?.quota || 0
  let completionAccent = 'bg-rose-500/70'
  if (completionRate >= 99) {
    completionAccent = 'bg-emerald-500/70'
  } else if (completionRate >= 95) {
    completionAccent = 'bg-amber-500/70'
  }

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatSparkline series={stats?.series} />
      <StatBadge
        label={t('Requests')}
        value={formatCompactNumber(requests)}
        accent='bg-sky-500/70'
        title={`${t('Requests')}: ${requests} · ${t('Summary metrics count requests')}`}
      />
      <StatBadge
        label={t('Completion Rate')}
        value={formatPercent(completionRate)}
        accent={completionAccent}
        title={`${t('Failed')}: ${failed}`}
      />
      <StatBadge
        label={t('Failed')}
        value={formatCompactNumber(failed)}
        accent={failed > 0 ? 'bg-rose-500/70' : 'bg-slate-400/70'}
        title={`${t('Error Rate')}: ${formatPercent(stats?.error_rate ?? 0)} · ${t('Retried')}: ${retried}`}
      />
      <StatBadge
        label={t('Usage')}
        value={sensitiveVisible ? formatLogQuota(quota) : '••••'}
        accent='bg-sky-500/70'
      />
      <StatBadge
        label={t('Token')}
        value={formatCompactNumber(stats?.tokens ?? 0)}
        accent='bg-slate-400/70'
        title={`${t('Input')}: ${formatCompactNumber(stats?.prompt_tokens ?? 0)} · ${t('Output')}: ${formatCompactNumber(stats?.completion_tokens ?? 0)}`}
      />
      <StatBadge
        label='P95'
        value={formatUseTime(stats?.p95 ?? 0)}
        accent='bg-violet-500/70'
        title={`P50 ${formatUseTime(stats?.p50 ?? 0)} · P99 ${formatUseTime(stats?.p99 ?? 0)}`}
      />
      <StatBadge
        label='RPM / TPM'
        value={`${stats?.rpm || 0} / ${formatCompactNumber(stats?.tpm || 0)}`}
        accent='bg-slate-400/70'
        title={t('Requests and tokens in the last minute')}
      />
    </div>
  )
}
