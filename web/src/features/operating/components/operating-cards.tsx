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
import {
  ArrowDownToLine,
  ArrowUpFromLine,
  Coins,
  Receipt,
  Trophy,
  WalletCards,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { formatLocalCurrencyAmount } from '@/lib/currency'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { cn } from '@/lib/utils'

import {
  OPERATING_EXPENSE_CATEGORY_LABEL_KEYS,
  OPERATING_GRANT_LABEL_KEYS,
  OPERATING_GRANT_ORDER,
} from '../constants'
import {
  useDeleteOperatingExpense,
  useOperatingExpenses,
} from '../hooks/use-operating'
import type { OperatingRange, OperatingSummary } from '../types'

interface OperatingCardsProps {
  summary?: OperatingSummary
  range: OperatingRange
  loading: boolean
}

interface StatItem {
  key: string
  title: string
  value: string
  desc: string
  icon: typeof Coins
  iconTone: IconBadgeTone
  negative?: boolean
}

/** 与数据面板一致：一条带分隔线的横向指标带，而不是独立卡片。 */
export function OperatingSummaryCards(props: OperatingCardsProps) {
  const { t } = useTranslation()
  const summary = props.summary

  const netInflow = summary
    ? summary.revenue.amount_cents - summary.cost.amount_cents
    : 0

  const revenueDesc = () => {
    if (!summary) return t('Successful top-ups')
    if (summary.revenue.mixed_currency) {
      return t('{{count}} orders, converted from multiple currencies', {
        count: summary.revenue.orders,
      })
    }
    return t('{{count}} orders, {{quota}} credited', {
      count: summary.revenue.orders,
      quota: formatQuota(summary.revenue.credited_quota),
    })
  }

  const items: StatItem[] = [
    {
      key: 'net_inflow',
      title: t('Cash inflow'),
      value: summary ? formatLocalCurrencyAmount(netInflow / 100) : '--',
      desc: t('Paid amount minus recorded expenses'),
      icon: netInflow < 0 ? ArrowDownToLine : ArrowUpFromLine,
      iconTone: netInflow < 0 ? 'warning' : 'success',
      negative: netInflow < 0,
    },
    {
      key: 'revenue',
      title: t('Revenue'),
      value: summary
        ? formatLocalCurrencyAmount(summary.revenue.amount_cents / 100)
        : '--',
      desc: revenueDesc(),
      icon: Coins,
      iconTone: 'chart-1',
    },
    {
      key: 'cost',
      title: t('Expenses'),
      value: summary
        ? formatLocalCurrencyAmount(summary.cost.amount_cents / 100)
        : '--',
      desc: summary
        ? t('{{count}} records', { count: summary.cost.entries })
        : t('Recorded expenses'),
      icon: Receipt,
      iconTone: 'chart-4',
    },
    {
      key: 'consumption',
      title: t('Consumption'),
      value: summary ? formatQuota(summary.consumption.quota) : '--',
      desc: summary
        ? t('{{count}} users consumed', {
            count: summary.consumption.users,
          })
        : t('Consumed quota'),
      icon: Trophy,
      iconTone: 'chart-3',
    },
    {
      key: 'balance',
      title: t('Current balance'),
      value: summary ? formatQuota(summary.balance.quota) : '--',
      desc: summary
        ? t('Snapshot of {{count}} users at {{time}}', {
            count: summary.balance.users,
            time: formatTimestamp(summary.balance.snapshot_at),
          })
        : t('Remaining quota'),
      icon: WalletCards,
      iconTone: 'chart-2',
    },
  ]

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='divide-border/60 grid min-w-0 grid-cols-2 divide-x sm:grid-cols-3 lg:grid-cols-5'>
        {items.map((item, idx) => {
          const Icon = item.icon
          let valueContent
          if (props.loading) {
            valueContent = (
              <div className='mt-1 flex flex-col gap-1 sm:mt-2 sm:gap-1.5'>
                <Skeleton className='h-5 w-16 sm:h-7 sm:w-20' />
                <Skeleton className='hidden h-3.5 w-28 md:block' />
              </div>
            )
          } else {
            valueContent = (
              <>
                <div
                  className={cn(
                    'mt-1 max-w-full truncate font-mono text-base leading-tight font-bold tracking-tight tabular-nums sm:mt-2 sm:text-2xl sm:leading-normal',
                    item.negative ? 'text-destructive' : 'text-foreground'
                  )}
                  title={item.value}
                >
                  {item.value}
                </div>
                <div className='text-muted-foreground/60 mt-1 hidden text-xs md:block'>
                  {item.desc}
                </div>
              </>
            )
          }

          return (
            <div
              key={item.key}
              role='group'
              aria-label={item.title}
              aria-busy={props.loading}
              className={cn(
                'min-w-0 px-2.5 py-1.5 sm:px-3 sm:py-4 xl:px-4',
                idx === items.length - 1 &&
                  items.length % 2 !== 0 &&
                  'col-span-2 sm:col-span-1'
              )}
            >
              <div className='flex min-w-0 items-center gap-1.5 sm:gap-2'>
                <IconBadge
                  tone={item.iconTone}
                  size='stat'
                  className='size-4 rounded-sm sm:size-7 sm:rounded-md [&>svg]:size-2.5 sm:[&>svg]:size-3.5'
                >
                  <Icon />
                </IconBadge>
                <div
                  className='text-muted-foreground truncate text-[11px] leading-4 font-medium tracking-wide uppercase sm:text-xs sm:tracking-wider'
                  title={item.title}
                >
                  {item.title}
                </div>
              </div>
              {valueContent}
            </div>
          )
        })}
      </div>
    </div>
  )
}

export function OperatingGrantTable(props: OperatingCardsProps) {
  const { t } = useTranslation()
  const summary = props.summary

  const grants = OPERATING_GRANT_ORDER.map((key) => {
    const found = summary?.grants.find((grant) => grant.key === key)
    return (
      found ?? {
        key,
        count: 0,
        quota: 0,
        accuracy: key === 'signup' || key === 'invite' ? 'estimated' : 'exact',
      }
    )
  })
  const total = grants.reduce((sum, grant) => sum + grant.quota, 0)

  const meta = `${t('Total:')} ${formatQuota(total)}`

  const renderBody = () => {
    if (props.loading) {
      return <LoadingState inline className='justify-center py-14' />
    }
    return (
      <div className='overflow-x-auto'>
        <table className='w-full text-sm'>
          <thead>
            <tr className='text-muted-foreground border-border/60 border-b text-left text-xs'>
              <th className='py-2 pr-3 pl-3 font-medium sm:pl-5'>
                {t('Source')}
              </th>
              <th className='py-2 pr-3 text-right font-medium'>
                {t('Records')}
              </th>
              <th className='py-2 pr-3 text-right font-medium'>
                {t('Issued quota')}
              </th>
              <th className='py-2 pr-3 font-medium sm:pr-5'>{t('Accuracy')}</th>
            </tr>
          </thead>
          <tbody>
            {grants.map((grant) => (
              <tr
                key={grant.key}
                className='border-border/40 hover:bg-muted/30 border-b'
              >
                <td className='py-2.5 pr-3 pl-3 sm:pl-5'>
                  {t(OPERATING_GRANT_LABEL_KEYS[grant.key] ?? grant.key)}
                </td>
                <td className='py-2.5 pr-3 text-right tabular-nums'>
                  {grant.count}
                </td>
                <td className='py-2.5 pr-3 text-right tabular-nums'>
                  {formatQuota(grant.quota)}
                </td>
                <td className='text-muted-foreground py-2.5 pr-3 text-xs sm:pr-5'>
                  {grant.accuracy === 'exact'
                    ? t('Exact')
                    : t('Parsed from log text')}
                </td>
              </tr>
            ))}
            <tr className='font-medium'>
              <td className='py-2.5 pr-3 pl-3 sm:pl-5'>{t('Total')}</td>
              <td />
              <td className='py-2.5 pr-3 text-right tabular-nums'>
                {formatQuota(total)}
              </td>
              <td />
            </tr>
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex items-center gap-2'>
          <IconBadge tone='success' size='sm'>
            <Coins />
          </IconBadge>
          <div className='text-sm font-semibold'>{t('Credits issued')}</div>
          <span className='text-muted-foreground text-xs'>{meta}</span>
        </div>
      </div>
      <p className='text-muted-foreground/60 border-b px-3 py-2 text-xs sm:px-5'>
        {t(
          'How much quota was issued in this range, grouped by source. It does not infer the origin of the balance a user still holds.'
        )}
      </p>
      {renderBody()}
    </div>
  )
}

export function OperatingExpenseTable(props: OperatingCardsProps) {
  const { t } = useTranslation()
  const expensesQuery = useOperatingExpenses(props.range)
  const deleteMutation = useDeleteOperatingExpense()
  // 支出是财务记录，删除前必须二次确认。
  const [pendingDelete, setPendingDelete] = useState<number | null>(null)

  const renderBody = () => {
    if (expensesQuery.isLoading) {
      return <LoadingState inline className='justify-center py-14' />
    }
    const expenses = expensesQuery.data ?? []
    if (expenses.length === 0) {
      return (
        <div className='text-muted-foreground py-14 text-center text-sm'>
          {t('No expenses recorded in this range')}
        </div>
      )
    }
    return (
      <div className='overflow-x-auto'>
        <table className='w-full text-sm'>
          <thead>
            <tr className='text-muted-foreground border-border/60 border-b text-left text-xs'>
              <th className='py-2 pr-3 pl-3 font-medium sm:pl-5'>
                {t('Date')}
              </th>
              <th className='py-2 pr-3 font-medium'>{t('Category')}</th>
              <th className='py-2 pr-3 text-right font-medium'>
                {t('Amount')}
              </th>
              <th className='py-2 pr-3 font-medium'>{t('Note')}</th>
              <th className='py-2 pr-3 font-medium sm:pr-5'>{t('Actions')}</th>
            </tr>
          </thead>
          <tbody>
            {expenses.map((expense) => (
              <tr
                key={expense.id}
                className='border-border/40 hover:bg-muted/30 border-b'
              >
                <td className='py-2.5 pr-3 pl-3 tabular-nums sm:pl-5'>
                  {formatTimestamp(expense.occurred_at)}
                </td>
                <td className='py-2.5 pr-3'>
                  {t(
                    OPERATING_EXPENSE_CATEGORY_LABEL_KEYS[expense.category] ??
                      expense.category
                  )}
                </td>
                <td className='py-2.5 pr-3 text-right tabular-nums'>
                  {formatLocalCurrencyAmount(expense.amount_cents / 100)}
                </td>
                <td className='text-muted-foreground max-w-56 truncate py-2.5 pr-3'>
                  {expense.note || '-'}
                </td>
                <td className='py-2.5 pr-3 sm:pr-5'>
                  <Button
                    variant='ghost'
                    size='sm'
                    className='text-muted-foreground hover:text-destructive h-7 px-2'
                    disabled={deleteMutation.isPending}
                    onClick={() => setPendingDelete(expense.id)}
                  >
                    {t('Delete')}
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  return (
    <>
      <div className='overflow-hidden rounded-lg border'>
        <div className='flex w-full flex-col gap-1.5 border-b px-3 py-2 sm:gap-3 sm:px-5 sm:py-3 lg:flex-row lg:items-center lg:justify-between'>
          <div className='flex items-center gap-2'>
            <IconBadge tone='chart-4' size='sm'>
              <Receipt />
            </IconBadge>
            <div className='text-sm font-semibold'>{t('Expense records')}</div>
          </div>
        </div>
        <p className='text-muted-foreground/60 border-b px-3 py-2 text-xs sm:px-5'>
          {t(
            'Expenses recorded by an administrator. Nothing is read from upstream providers automatically.'
          )}
        </p>
        {renderBody()}
      </div>
      <ConfirmDialog
        destructive
        open={pendingDelete !== null}
        onOpenChange={(open) => {
          if (!open) setPendingDelete(null)
        }}
        title={t('Delete this expense record?')}
        desc={t('The expense will be removed from this period total.')}
        confirmText={t('Delete')}
        handleConfirm={() => {
          if (pendingDelete !== null) deleteMutation.mutate(pendingDelete)
          setPendingDelete(null)
        }}
      />
    </>
  )
}
