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
import { useNavigate } from '@tanstack/react-router'
import {
  Activity,
  ArrowDownRight,
  ArrowUpRight,
  ExternalLink,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getCurrencyDisplay } from '@/lib/currency'
import {
  formatCompactNumber,
  formatPercent,
  formatQuota,
  formatUseTime,
  quotaUnitsToDollars,
} from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { computeTimeRange, formatChartTime } from '@/lib/time'
import { cn } from '@/lib/utils'

import {
  getLogHealthSummary,
  getUserQuotaDataByUsers,
  getUserQuotaDates,
} from '../../api'
import { buildQueryParams, getDefaultDays } from '../../lib'
import type {
  DashboardFilters,
  DashboardLogHealthSummary,
  QuotaDataItem,
} from '../../types'

type HealthChartMode =
  | 'cost'
  | 'requests'
  | 'failed'
  | 'failed4xx'
  | 'p95'
  | 'interrupted'

interface AdminHealthPanelProps {
  filters?: DashboardFilters
}

interface CostRankItem {
  label: string
  cost: number
  count: number
}

function percentChange(current: number, previous: number | undefined) {
  if (previous == null || previous <= 0) return undefined
  return ((current - previous) / previous) * 100
}

function percentagePointChange(current: number, previous: number | undefined) {
  if (previous == null) return undefined
  return current - previous
}

function getPreviousCompletionRate(
  summary: DashboardLogHealthSummary | undefined
) {
  if (!summary || summary.requests <= 0) return undefined
  return (summary.success_requests / summary.requests) * 100
}

function topCostItems(
  rows: QuotaDataItem[],
  key: 'model_name' | 'username'
): CostRankItem[] {
  const values = new Map<string, CostRankItem>()
  for (const row of rows) {
    const label = row[key] || 'Unknown'
    const item = values.get(label) || { label, cost: 0, count: 0 }
    item.cost += Number(row.quota) || 0
    item.count += Number(row.count) || 0
    values.set(label, item)
  }
  return [...values.values()]
    .sort((a, b) => b.cost - a.cost)
    .slice(0, 5)
}

function CostRankItems(props: {
  items: CostRankItem[]
  onSelect: (label: string) => void
}) {
  const { t } = useTranslation()
  const maxCost = Math.max(1, ...props.items.map((item) => item.cost))
  if (props.items.length === 0) {
    return (
      <div className='text-muted-foreground/60 py-2 text-[11px]'>
        {t('No data available')}
      </div>
    )
  }
  return (
    <div className='space-y-1.5'>
      {props.items.map((item) => (
        <button
          key={item.label}
          type='button'
          className='hover:bg-muted/50 -mx-1 w-full rounded px-1 py-0.5 text-left'
          onClick={() => props.onSelect(item.label)}
          title={item.label}
        >
          <div className='flex items-center justify-between gap-2 text-[11px]'>
            <span className='text-foreground/85 min-w-0 truncate'>
              {item.label}
            </span>
            <span className='text-foreground/85 shrink-0 font-mono font-medium'>
              {formatQuota(item.cost)}
            </span>
          </div>
          <div className='bg-muted mt-1 h-1 overflow-hidden rounded-full'>
            <div
              className='h-full rounded-full bg-sky-500/70'
              style={{ width: `${(item.cost / maxCost) * 100}%` }}
            />
          </div>
          <div className='text-muted-foreground/50 mt-0.5 text-[10px]'>
            {formatCompactNumber(item.count)} {t('requests')}
          </div>
        </button>
      ))}
    </div>
  )
}

function HealthTrendChart(props: {
  series: DashboardLogHealthSummary['series']
  mode: HealthChartMode
}) {
  const { t } = useTranslation()
  const { meta } = getCurrencyDisplay()
  const series = props.series
  if (series.length === 0) {
    return (
      <div className='text-muted-foreground flex h-[220px] items-center justify-center text-xs'>
        {t('No request health data')}
      </div>
    )
  }

  const width = 840
  const height = 220
  const margin = { top: 12, right: 12, bottom: 26, left: 48 }
  const innerW = width - margin.left - margin.right
  const innerH = height - margin.top - margin.bottom
  const step = innerW / series.length
  const barW = Math.max(2, Math.min(22, step * 0.62))
  const costValue = (bucket: DashboardLogHealthSummary['series'][number]) =>
    quotaUnitsToDollars(bucket.quota)
  const bucketSeconds = series.length > 1 ? series[1].t - series[0].t : 3600
  const granularity = bucketSeconds >= 86400 ? 'day' : 'hour'
  const isLine = props.mode === 'p95'

  const values = series.map((bucket) => {
    switch (props.mode) {
      case 'cost':
        return costValue(bucket)
      case 'failed':
        return bucket.failed
      case 'failed4xx':
        return bucket.failed_4xx
      case 'p95':
        return bucket.p95
      case 'interrupted':
        return bucket.interrupted
      default:
        return bucket.success + bucket.failed
    }
  })
  const maxValue =
    props.mode === 'p95'
      ? Math.max(1, Math.max(...values) * 1.15)
      : Math.max(1, ...values)
  const y = (value: number) =>
    margin.top + innerH - (value / maxValue) * innerH

  const formatAxisValue = (value: number) => {
    if (props.mode === 'cost') {
      return meta.kind === 'tokens'
        ? formatCompactNumber(value)
        : `${meta.symbol}${formatCompactNumber(value)}`
    }
    if (props.mode === 'p95') return `${value.toFixed(1)}s`
    return formatCompactNumber(value)
  }

  const firstBucket = series[0]
  const lastBucket = series.at(-1)
  const bucketTitle = (bucket: DashboardLogHealthSummary['series'][number]) =>
    [
      formatChartTime(bucket.t, granularity),
      `${t('Cost')} ${formatQuota(bucket.quota)}`,
      `${t('Requests')} ${bucket.success + bucket.failed}`,
      `${t('Failed')} ${bucket.failed}`,
      `4xx ${bucket.failed_4xx}`,
      `${t('Interrupted')} ${bucket.interrupted}`,
      `P95 ${formatUseTime(bucket.p95)}`,
    ].join(' · ')

  const bars: ReactNode[] = []
  const points: Array<{ x: number; y: number; title: string }> = []
  series.forEach((bucket, index) => {
    const x = margin.left + index * step + (step - barW) / 2
    const title = bucketTitle(bucket)
    if (props.mode === 'requests') {
      let cursor = margin.top + innerH
      const segments: ReactNode[] = []
      const entries = [
        [bucket.success, '#10b981'],
        [bucket.failed, '#ef4444'],
      ] as const
      for (const [value, color] of entries) {
        if (!value) continue
        const barHeight = (value / maxValue) * innerH
        cursor -= barHeight
        segments.push(
          <rect
            key={`${bucket.t}-${color}`}
            x={x}
            y={cursor}
            width={barW}
            height={barHeight}
            fill={color}
            opacity={0.85}
          />
        )
      }
      bars.push(
        <g key={bucket.t}>
          <title>{title}</title>
          {segments}
        </g>
      )
      return
    }
    if (isLine) {
      points.push({
        x: margin.left + index * step + step / 2,
        y: y(bucket.p95),
        title,
      })
      return
    }
    let color = '#3b82f6'
    if (props.mode === 'failed' || props.mode === 'failed4xx') {
      color = '#ef4444'
    } else if (props.mode === 'interrupted') {
      color = '#f59e0b'
    }
    let value = bucket.interrupted
    if (props.mode === 'cost') {
      value = costValue(bucket)
    } else if (props.mode === 'failed') {
      value = bucket.failed
    } else if (props.mode === 'failed4xx') {
      value = bucket.failed_4xx
    }
    const barHeight = (value / maxValue) * innerH
    bars.push(
      <g key={bucket.t}>
        <title>{title}</title>
        <rect
          x={x}
          y={margin.top + innerH - barHeight}
          width={barW}
          height={Math.max(0, barHeight)}
          fill={color}
          opacity={0.85}
        />
      </g>
    )
  })

  let linePath = ''
  let areaPath = ''
  if (isLine && points.length > 0) {
    linePath = points
      .map((point, index) => `${index === 0 ? 'M' : 'L'}${point.x} ${point.y}`)
      .join(' ')
    const first = points[0]
    const last = points.at(-1)
    if (first && last) {
      areaPath = `${linePath} L${last.x} ${margin.top + innerH} L${first.x} ${margin.top + innerH} Z`
    }
  }

  const grid: ReactNode[] = []
  const ticks = 4
  for (let index = 0; index <= ticks; index++) {
    const value = (maxValue / ticks) * index
    const lineY = y(value)
    grid.push(
      <line
        key={`grid-${index}`}
        x1={margin.left}
        x2={width - margin.right}
        y1={lineY}
        y2={lineY}
        stroke='currentColor'
        opacity={0.12}
      />
    )
    grid.push(
      <text
        key={`grid-label-${index}`}
        x={margin.left - 7}
        y={lineY + 3.5}
        textAnchor='end'
        fill='currentColor'
        opacity={0.55}
        fontSize={9.5}
      >
        {formatAxisValue(value)}
      </text>
    )
  }

  const labelCount = Math.min(7, series.length)
  const xLabels: ReactNode[] = []
  for (let index = 0; index < labelCount; index++) {
    const bucketIndex = Math.round(
      (index / Math.max(1, labelCount - 1)) * (series.length - 1)
    )
    const bucket = series[bucketIndex]
    const x = margin.left + bucketIndex * step + step / 2
    xLabels.push(
      <text
        key={`x-label-${bucketIndex}`}
        x={x}
        y={height - 7}
        textAnchor='middle'
        fill='currentColor'
        opacity={0.55}
        fontSize={9.5}
      >
        {formatChartTime(bucket.t, granularity)}
      </text>
    )
  }

  const legendLabel = {
    cost: t('Cost'),
    requests: '',
    failed: t('Failed requests'),
    failed4xx: '4xx',
    p95: 'P95',
    interrupted: t('Interrupted'),
  }[props.mode]

  return (
    <div className='text-muted-foreground'>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className='h-[220px] w-full'
        role='img'
        aria-label={t('Operations trend')}
      >
        {grid}
        {bars}
        {areaPath && <path d={areaPath} fill='#f59e0b' opacity='0.1' />}
        {linePath && (
          <path
            d={linePath}
            fill='none'
            stroke='#f59e0b'
            strokeWidth={2}
            strokeLinejoin='round'
            strokeLinecap='round'
          />
        )}
        {isLine &&
          points.map((point) => (
            <circle
              key={`${point.x}-${point.y}`}
              cx={point.x}
              cy={point.y}
              r={2.2}
              fill='#f59e0b'
            >
              <title>{point.title}</title>
            </circle>
          ))}
        {xLabels}
      </svg>
      <div className='mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 px-1 text-[11px]'>
        {props.mode === 'requests' ? (
          <>
            <span className='inline-flex items-center gap-1.5'>
              <span className='size-2 rounded-[2px] bg-emerald-500' />
              {t('Success')}
            </span>
            <span className='inline-flex items-center gap-1.5'>
              <span className='size-2 rounded-[2px] bg-red-500' />
              {t('Failed')}
            </span>
          </>
        ) : (
          <span className='inline-flex items-center gap-1.5'>
            <span
              className={cn(
                'size-2 rounded-[2px]',
                props.mode === 'cost' && 'bg-sky-500',
                (props.mode === 'failed' || props.mode === 'failed4xx') &&
                  'bg-red-500',
                (props.mode === 'p95' || props.mode === 'interrupted') &&
                  'bg-amber-500'
              )}
            />
            {legendLabel}
          </span>
        )}
        {firstBucket && lastBucket && (
          <span className='text-muted-foreground/50 ml-auto'>
            {formatChartTime(firstBucket.t, granularity)} -{' '}
            {formatChartTime(lastBucket.t, granularity)}
          </span>
        )}
      </div>
    </div>
  )
}

export function AdminHealthPanel(props: AdminHealthPanelProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [mode, setMode] = useState<HealthChartMode>('requests')
  const [costTab, setCostTab] = useState<'models' | 'users'>('models')
  const timeRange = useMemo(
    () =>
      computeTimeRange(
        getDefaultDays(props.filters?.time_granularity),
        props.filters?.start_timestamp,
        props.filters?.end_timestamp
      ),
    [props.filters]
  )
  const duration = timeRange.end_timestamp - timeRange.start_timestamp
  const healthQuery = useQuery({
    queryKey: [
      'dashboard',
      'decision-health',
      timeRange.start_timestamp,
      timeRange.end_timestamp,
      props.filters?.username,
    ],
    queryFn: async () => {
      const currentResult = requireServerSuccess(
        await getLogHealthSummary({
          start_timestamp: timeRange.start_timestamp,
          end_timestamp: timeRange.end_timestamp,
          username: props.filters?.username || undefined,
        })
      )
      let previous: DashboardLogHealthSummary | undefined
      if (duration > 0) {
        try {
          const previousResult = requireServerSuccess(
            await getLogHealthSummary({
              start_timestamp: timeRange.start_timestamp - duration,
              end_timestamp: timeRange.start_timestamp,
              username: props.filters?.username || undefined,
            })
          )
          previous = previousResult.data
        } catch {
          previous = undefined
        }
      }
      return { current: currentResult.data, previous }
    },
    staleTime: 60 * 1000,
    retry: false,
  })
  const modelQuery = useQuery({
    queryKey: [
      'dashboard',
      'decision-models',
      timeRange.start_timestamp,
      timeRange.end_timestamp,
      props.filters?.username,
    ],
    queryFn: async () =>
      requireServerSuccess(
        await getUserQuotaDates(
          buildQueryParams(timeRange, {
            time_granularity: props.filters?.time_granularity,
            username: props.filters?.username,
          }),
          true
        )
      ),
    staleTime: 60 * 1000,
    retry: false,
  })
  const userQuery = useQuery({
    queryKey: [
      'dashboard',
      'decision-users',
      timeRange.start_timestamp,
      timeRange.end_timestamp,
    ],
    queryFn: async () =>
      requireServerSuccess(
        await getUserQuotaDataByUsers({
          start_timestamp: timeRange.start_timestamp,
          end_timestamp: timeRange.end_timestamp,
        })
      ),
    staleTime: 60 * 1000,
    retry: false,
  })

  const current = healthQuery.data?.current
  const previous = healthQuery.data?.previous
  const requests = current?.requests ?? 0
  const success = current?.success_requests ?? 0
  const failed = current?.failed_requests ?? 0
  const failed4xx = current?.failed_4xx ?? 0
  const failed5xx = current?.failed_5xx ?? 0
  const interrupted = current?.interrupted_requests ?? 0
  const completionRate = requests > 0 ? (success / requests) * 100 : 0
  const previousCompletionRate = getPreviousCompletionRate(previous)
  const errorRate = current?.error_rate ?? 0
  const p95 = current?.p95 ?? 0
  const p50 = current?.p50 ?? 0
  const quota = current?.quota ?? 0
  const statusCodes = current?.status_codes ?? {}
  const topModels = useMemo(
    () => topCostItems(modelQuery.data?.data ?? [], 'model_name'),
    [modelQuery.data]
  )
  const topUsers = useMemo(
    () => topCostItems(userQuery.data?.data ?? [], 'username'),
    [userQuery.data]
  )

  const kpis = [
    {
      label: t('Cost'),
      value: formatQuota(quota),
      delta: percentChange(quota, previous?.quota),
      unit: 'pct' as const,
      positiveIsGood: true,
      neutral: true,
      sub: `${formatCompactNumber(requests)} ${t('requests')}`,
    },
    {
      label: t('Requests'),
      value: formatCompactNumber(requests),
      delta: percentChange(requests, previous?.requests),
      unit: 'pct' as const,
      positiveIsGood: true,
      neutral: true,
      sub: `${formatCompactNumber(success)} ${t('Success')} · ${formatCompactNumber(failed)} ${t('Failed')}`,
    },
    {
      label: t('Completion Rate'),
      value: formatPercent(completionRate),
      delta: percentagePointChange(completionRate, previousCompletionRate),
      unit: 'pp' as const,
      positiveIsGood: true,
      neutral: false,
      sub: `${formatCompactNumber(failed)} ${t('Failed')}`,
    },
    {
      label: '4xx',
      value: formatCompactNumber(failed4xx),
      delta: percentChange(failed4xx, previous?.failed_4xx),
      unit: 'pct' as const,
      positiveIsGood: false,
      neutral: false,
      sub: `403 ${formatCompactNumber(statusCodes['403'] ?? 0)} · 401 ${formatCompactNumber(statusCodes['401'] ?? 0)}`,
    },
    {
      label: t('Failed'),
      value: formatCompactNumber(failed),
      delta: percentChange(failed, previous?.failed_requests),
      unit: 'pct' as const,
      positiveIsGood: false,
      neutral: false,
      sub: `5xx ${formatCompactNumber(failed5xx)} · ${formatCompactNumber(current?.error_rows ?? 0)} ${t('errors')}`,
    },
    {
      label: t('Error Rate'),
      value: formatPercent(errorRate),
      delta: percentagePointChange(errorRate, previous?.error_rate),
      unit: 'pp' as const,
      positiveIsGood: false,
      neutral: false,
      sub: `${formatCompactNumber(failed)} ${t('Failed')}`,
    },
    {
      label: 'P95',
      value: formatUseTime(p95),
      delta: percentChange(p95, previous?.p95),
      unit: 'pct' as const,
      positiveIsGood: false,
      neutral: false,
      sub: `P50 ${formatUseTime(p50)}`,
    },
    {
      label: t('Interrupted'),
      value: formatCompactNumber(interrupted),
      delta: percentChange(interrupted, previous?.interrupted_requests),
      unit: 'pct' as const,
      positiveIsGood: false,
      neutral: false,
      sub: `${formatCompactNumber(requests)} ${t('requests')}`,
    },
  ]

  const openModelLogs = (model: string) => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        model,
        startTime: timeRange.start_timestamp * 1000,
        endTime: timeRange.end_timestamp * 1000,
        view: 'sessions',
        page: 1,
      },
    })
  }
  const openUserLogs = (username: string) => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        username,
        startTime: timeRange.start_timestamp * 1000,
        endTime: timeRange.end_timestamp * 1000,
        view: 'sessions',
        page: 1,
      },
    })
  }
  const handleViewFailedLogs = () => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        type: ['5'],
        startTime: timeRange.start_timestamp * 1000,
        endTime: timeRange.end_timestamp * 1000,
        username: props.filters?.username || undefined,
        view: 'sessions',
        page: 1,
      },
    })
  }

  let body: ReactNode
  if (healthQuery.isLoading) {
    body = (
      <div className='space-y-3 p-4'>
        <div className='grid grid-cols-2 gap-3 sm:grid-cols-4 lg:grid-cols-8'>
          {Array.from({ length: 8 }, (_, index) => (
            <Skeleton key={index} className='h-16 rounded-md' />
          ))}
        </div>
        <Skeleton className='h-[300px] rounded-md' />
      </div>
    )
  } else if (healthQuery.isError) {
    body = (
      <div className='text-muted-foreground px-4 py-8 text-center text-xs'>
        {t('No request health data')}
      </div>
    )
  } else {
    body = (
      <>
        <div className='divide-border/60 grid grid-cols-2 divide-x divide-y sm:grid-cols-4 lg:grid-cols-8'>
          {kpis.map((kpi) => {
            const delta = kpi.delta
            const isUp = delta != null && delta > 0.05
            const isDown = delta != null && delta < -0.05
            let isGood: boolean | null = null
            if (!kpi.neutral) {
              if (isUp) {
                isGood = kpi.positiveIsGood
              } else if (isDown) {
                isGood = !kpi.positiveIsGood
              }
            }
            let deltaClass = 'text-muted-foreground'
            if (isGood === true) {
              deltaClass = 'text-emerald-600 dark:text-emerald-400'
            } else if (isGood === false) {
              deltaClass = 'text-rose-600 dark:text-rose-400'
            }
            const DeltaIcon = isUp ? ArrowUpRight : ArrowDownRight
            return (
              <div key={kpi.label} className='min-w-0 px-3 py-3'>
                <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wide uppercase'>
                  {kpi.label}
                </div>
                <div className='mt-1 flex flex-wrap items-baseline gap-x-2 gap-y-0.5'>
                  <span className='font-mono text-xl font-bold tabular-nums'>
                    {kpi.value}
                  </span>
                  {delta != null && (
                    <span
                      className={cn(
                        'inline-flex items-center text-[11px] font-medium',
                        deltaClass
                      )}
                      title={t('vs previous period')}
                    >
                      <DeltaIcon className='size-3' />
                      {Math.abs(delta).toFixed(1)}
                      {kpi.unit === 'pp' ? 'pp' : '%'}
                    </span>
                  )}
                </div>
                <div className='text-muted-foreground/60 mt-0.5 truncate text-[11px]'>
                  {kpi.sub}
                </div>
              </div>
            )
          })}
        </div>

        <div className='grid gap-3 border-t px-3 py-3 sm:px-4 lg:grid-cols-[minmax(0,1fr)_280px]'>
          <div className='min-w-0'>
            <div className='mb-2 flex flex-wrap items-center justify-between gap-2'>
              <div className='text-xs font-semibold'>{t('Operations trend')}</div>
              <div className='bg-muted/60 inline-flex h-7 flex-wrap rounded-lg border p-0.5'>
                {(
                  [
                    ['cost', t('Cost')],
                    ['requests', t('Requests')],
                    ['failed', t('Failed')],
                    ['failed4xx', '4xx'],
                    ['p95', 'P95'],
                    ['interrupted', t('Interrupted')],
                  ] as const
                ).map(([value, label]) => (
                  <button
                    key={value}
                    type='button'
                    onClick={() => setMode(value)}
                    className={cn(
                      'rounded-md px-2 text-[11px] font-medium transition-colors',
                      mode === value
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    {label}
                  </button>
                ))}
              </div>
            </div>
            <HealthTrendChart series={current?.series ?? []} mode={mode} />
          </div>
          <div className='min-w-0 rounded-md border px-3 py-2.5'>
            <div className='mb-2 flex items-center justify-between gap-2'>
              <span className='text-muted-foreground text-[11px] font-semibold tracking-wide uppercase'>
                {t('Cost destination')}
              </span>
              <div className='bg-muted/60 inline-flex h-6 rounded-md border p-0.5'>
                {(['models', 'users'] as const).map((tab) => (
                  <button
                    key={tab}
                    type='button'
                    onClick={() => setCostTab(tab)}
                    className={cn(
                      'rounded px-2 text-[10px] font-medium transition-colors',
                      costTab === tab
                        ? 'bg-background text-foreground shadow-sm'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    {t(tab === 'models' ? 'Models' : 'Users')}
                  </button>
                ))}
              </div>
            </div>
            {costTab === 'models' ? (
              <CostRankItems items={topModels} onSelect={openModelLogs} />
            ) : (
              <CostRankItems items={topUsers} onSelect={openUserLogs} />
            )}
          </div>
        </div>
      </>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-start gap-2 border-b px-4 py-3 sm:px-5'>
        <IconBadge tone='success' size='sm'>
          <Activity />
        </IconBadge>
        <div className='min-w-0'>
          <div className='text-sm font-semibold'>{t('Operations overview')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Cost, traffic and site health')}
          </div>
        </div>
        <Button
          variant='outline'
          size='sm'
          className='ms-auto shrink-0'
          onClick={handleViewFailedLogs}
        >
          <ExternalLink className='size-3.5' />
          {t('View failed logs')}
        </Button>
      </div>
      {body}
    </div>
  )
}