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
import { Coins, CreditCard, Eye } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { IconBadge } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatLocalCurrencyAmount } from '@/lib/currency'
import { formatQuota } from '@/lib/format'

import { useFreeQuotaStats } from '../../hooks/use-free-quota-stats'
import type { DashboardFilters } from '../../types'
import { FreeQuotaMethodBadge } from './free-quota-badge'
import { FreeQuotaDetailsDialog } from './free-quota-details-dialog'
import { FREE_QUOTA_SOURCE_LABELS } from './free-quota-labels'
import { OnlineTopupDetailsDialog } from './online-topup-details-dialog'

interface FreeQuotaPanelProps {
  filters?: DashboardFilters
}

export function FreeQuotaPanel(props: FreeQuotaPanelProps) {
  const { t } = useTranslation()
  const { stats, isLoading, isError, timeRange } = useFreeQuotaStats(
    props.filters
  )
  const [detailsOpen, setDetailsOpen] = useState(false)
  const [topupDetailsOpen, setTopupDetailsOpen] = useState(false)
  const visibleSources = (stats?.sources ?? []).filter(
    (source) => source.count > 0 && source.quota > 0
  )

  let body: ReactNode
  if (isLoading) {
    body = (
      <div className='grid grid-cols-2 gap-3 p-4 sm:grid-cols-3 lg:grid-cols-7'>
        {Array.from({ length: 7 }, (_, index) => (
          <Skeleton key={index} className='h-16 rounded-md' />
        ))}
      </div>
    )
  } else if (isError) {
    body = (
      <div className='text-muted-foreground px-4 py-6 text-center text-xs'>
        {t('No data available')}
      </div>
    )
  } else if (visibleSources.length === 0) {
    body = (
      <div className='text-muted-foreground px-4 py-6 text-center text-xs'>
        {t('No quota was consumed in this range')}
      </div>
    )
  } else {
    body = (
      <div className='divide-border/60 grid grid-cols-[repeat(auto-fit,minmax(9rem,1fr))] divide-x divide-y'>
        {visibleSources.map((source) => (
          <div key={source.key} className='min-w-0 px-3 py-3'>
            <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wide uppercase'>
              {t(FREE_QUOTA_SOURCE_LABELS[source.key] ?? source.key)}
            </div>
            <div className='mt-1 flex flex-wrap items-baseline gap-x-1.5 gap-y-0.5'>
              <span className='font-mono text-lg font-bold tabular-nums'>
                {formatQuota(source.quota)}
              </span>
              <FreeQuotaMethodBadge method={source.method} />
            </div>
            <div className='text-muted-foreground/60 mt-0.5 text-[11px]'>
              {source.count} {t('users')}
              {source.kind !== 'paid' && (
                <>
                  <span className='mx-1'>·</span>
                  {t('Granted')} {formatQuota(source.granted)}
                </>
              )}
            </div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-start gap-2 border-b px-4 py-3 sm:px-5'>
        <IconBadge tone='chart-3' size='sm'>
          <Coins />
        </IconBadge>
        <div className='min-w-0'>
          <div className='text-sm font-semibold'>{t('Quota consumption')}</div>
          <div className='text-muted-foreground text-xs'>
            {t('Actual consumption by source')}
          </div>
        </div>
        <div className='ms-auto flex items-center gap-3'>
          <Button
            variant='ghost'
            size='sm'
            className='text-muted-foreground hover:text-foreground shrink-0 gap-1'
            onClick={() => setDetailsOpen(true)}
          >
            <Eye className='size-3.5' />
            {t('View details')}
          </Button>
          <div className='text-right'>
            <div className='text-muted-foreground text-[11px]'>
              {t('Total consumption')}
            </div>
            <div className='font-mono text-lg font-bold tabular-nums'>
              {formatQuota(stats?.consumption_total ?? 0)}
            </div>
            <div className='text-muted-foreground/70 text-[11px]'>
              {t('Free portion')}{' '}
              {formatQuota(stats?.free_total ?? stats?.total ?? 0)}
            </div>
          </div>
        </div>
      </div>

      {body}

      {!isLoading && !isError && (
        <div className='text-muted-foreground flex flex-wrap items-center gap-x-3 gap-y-1 border-t px-4 py-2 text-[11px]'>
          <span className='text-foreground inline-flex items-center gap-1 font-medium'>
            <CreditCard className='size-3.5' />
            {t('Online top-up')}
          </span>
          <span>
            {stats?.online_topup_count ?? 0} {t('top-ups')}
          </span>
          {(stats?.online_topup_amount ?? 0) > 0 && (
            <span>
              {t('Paid')}{' '}
              {formatLocalCurrencyAmount(stats?.online_topup_amount)}
            </span>
          )}
          {(stats?.online_topup_count ?? 0) > 0 && (
            <span>
              {t('Credited')}{' '}
              {formatQuota(stats?.online_topup_quota ?? 0)}
            </span>
          )}
          <Button
            variant='ghost'
            size='sm'
            className='text-muted-foreground hover:text-foreground ms-auto h-6 shrink-0 gap-1 px-1.5'
            disabled={(stats?.online_topup_count ?? 0) === 0}
            onClick={() => setTopupDetailsOpen(true)}
          >
            <Eye className='size-3.5' />
            {t('Top-up details')}
          </Button>
        </div>
      )}

      {!isLoading && !isError && visibleSources.length > 0 && (
        <div className='text-muted-foreground space-y-0.5 border-t px-4 py-2 text-[11px] leading-relaxed'>
          <p>
            {t(
              'Only actually consumed free quota is counted; unused grants are excluded. Consumption is attributed to grants oldest-first.'
            )}
          </p>
          <p>
            {t(
              'Normal quota is total consumption minus consumed free quota.'
            )}
          </p>
        </div>
      )}

      <FreeQuotaDetailsDialog
        open={detailsOpen}
        onOpenChange={setDetailsOpen}
        stats={stats}
        timeRange={timeRange}
      />
      <OnlineTopupDetailsDialog
        open={topupDetailsOpen}
        onOpenChange={setTopupDetailsOpen}
        records={stats?.online_topup_records ?? []}
        timeRange={timeRange}
      />
    </div>
  )
}