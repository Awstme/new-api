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
import { useNavigate } from '@tanstack/react-router'
import { Copy } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatLocalCurrencyAmount } from '@/lib/currency'
import { formatQuota, formatTimestamp } from '@/lib/format'

import type { OnlineTopupRecord } from '../../types'

interface OnlineTopupDetailsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  records: OnlineTopupRecord[]
  timeRange?: {
    start_timestamp: number
    end_timestamp: number
  }
}

export function OnlineTopupDetailsDialog(props: OnlineTopupDetailsDialogProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const recordKeys = useMemo(
    () =>
      props.records.map(
        (record, index) =>
          record.request_id || `${record.user_id}:${record.created_at}:${index}`
      ),
    [props.records]
  )
  const [selected, setSelected] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (props.open) {
      setSelected(new Set(recordKeys))
    }
  }, [props.open, recordKeys])

  const selectedRecords = props.records.filter((_, index) =>
    selected.has(recordKeys[index])
  )
  const selectedAmount = selectedRecords.reduce(
    (total, record) => total + record.amount,
    0
  )
  const selectedQuota = selectedRecords.reduce(
    (total, record) => total + record.quota,
    0
  )
  const allSelected =
    recordKeys.length > 0 && selected.size === recordKeys.length
  const someSelected = selected.size > 0 && !allSelected

  const toggleAll = (checked: boolean) => {
    setSelected(checked ? new Set(recordKeys) : new Set())
  }

  const openUserLogs = (username: string) => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        username,
        view: 'sessions',
        page: 1,
        ...(props.timeRange
          ? {
              startTime: props.timeRange.start_timestamp * 1000,
              endTime: props.timeRange.end_timestamp * 1000,
            }
          : {}),
      },
    })
  }

  const copySelected = async () => {
    const header = [
      t('Time'),
      t('User'),
      t('Payment Method'),
      t('Paid'),
      t('Credited'),
    ].join('\t')
    const rows = selectedRecords.map((record) =>
      [
        formatTimestamp(record.created_at),
        record.username,
        record.payment_method || '—',
        record.amount > 0 ? formatLocalCurrencyAmount(record.amount) : '—',
        formatQuota(record.quota),
      ].join('\t')
    )
    const text = [header, ...rows].join('\n')
    try {
      await navigator.clipboard.writeText(text)
      toast.success(t('Copied to clipboard'))
    } catch {
      toast.error(t('Failed to copy'))
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Online top-up details')}
      description={t('Online recharge orders in the selected range.')}
      contentClassName='sm:max-w-4xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      {props.records.length === 0 ? (
        <div className='text-muted-foreground py-8 text-center text-xs'>
          {t('No online top-ups in this range')}
        </div>
      ) : (
        <>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <div className='text-muted-foreground text-xs'>
              {selectedRecords.length} {t('Selected')}
              <span className='mx-1'>·</span>
              {t('Paid')} {formatLocalCurrencyAmount(selectedAmount)}
              <span className='mx-1'>·</span>
              {t('Credited')} {formatQuota(selectedQuota)}
            </div>
            <Button
              variant='outline'
              size='sm'
              disabled={selectedRecords.length === 0}
              onClick={() => void copySelected()}
            >
              <Copy className='size-3.5' />
              {t('Copy selected')}
            </Button>
          </div>

          <div className='overflow-x-auto rounded-md border'>
            <table className='w-full min-w-[720px] text-xs'>
              <thead className='bg-muted/40 text-muted-foreground text-left'>
                <tr>
                  <th className='w-8 px-3 py-2'>
                    <Checkbox
                      checked={allSelected}
                      indeterminate={someSelected}
                      aria-label={t('Select all')}
                      onCheckedChange={(checked) => toggleAll(checked === true)}
                    />
                  </th>
                  <th className='px-3 py-2 font-medium'>{t('Time')}</th>
                  <th className='px-3 py-2 font-medium'>{t('User')}</th>
                  <th className='px-3 py-2 font-medium'>
                    {t('Payment Method')}
                  </th>
                  <th className='px-3 py-2 text-right font-medium'>
                    {t('Paid')}
                  </th>
                  <th className='px-3 py-2 text-right font-medium'>
                    {t('Credited')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {props.records.map((record, index) => {
                  const key = recordKeys[index]
                  return (
                    <tr
                      key={key}
                      className='border-border/60 border-t'
                    >
                      <td className='px-3 py-2'>
                        <Checkbox
                          checked={selected.has(key)}
                          aria-label={record.username}
                          onCheckedChange={(checked) =>
                            setSelected((previous) => {
                              const next = new Set(previous)
                              if (checked === true) next.add(key)
                              else next.delete(key)
                              return next
                            })
                          }
                        />
                      </td>
                      <td className='text-muted-foreground px-3 py-2 whitespace-nowrap'>
                        {formatTimestamp(record.created_at)}
                      </td>
                      <td className='px-3 py-2'>
                        <button
                          type='button'
                          title={t("View this user's logs")}
                          className='flex min-w-0 items-center gap-2 text-left'
                          onClick={() => openUserLogs(record.username)}
                        >
                          <Avatar className='ring-border/60 size-6 ring-1'>
                            <AvatarFallback
                              className='text-[11px] font-semibold'
                              style={getUserAvatarStyle(record.username)}
                            >
                              {getUserAvatarFallback(record.username)}
                            </AvatarFallback>
                          </Avatar>
                          <span className='truncate font-medium hover:underline'>
                            {record.username}
                          </span>
                        </button>
                      </td>
                      <td className='text-muted-foreground px-3 py-2'>
                        {record.payment_method || '—'}
                      </td>
                      <td className='px-3 py-2 text-right font-mono tabular-nums'>
                        {record.amount > 0
                          ? formatLocalCurrencyAmount(record.amount)
                          : '—'}
                      </td>
                      <td className='px-3 py-2 text-right font-mono tabular-nums'>
                        {formatQuota(record.quota)}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
              <tfoot className='bg-muted/30'>
                <tr className='border-border/60 border-t'>
                  <td className='px-3 py-2' />
                  <td className='px-3 py-2 font-semibold' colSpan={3}>
                    {t('Total')}
                  </td>
                  <td className='px-3 py-2 text-right font-mono font-semibold tabular-nums'>
                    {formatLocalCurrencyAmount(props.records.reduce(
                      (total, record) => total + record.amount,
                      0
                    ))}
                  </td>
                  <td className='px-3 py-2 text-right font-mono font-semibold tabular-nums'>
                    {formatQuota(
                      props.records.reduce(
                        (total, record) => total + record.quota,
                        0
                      )
                    )}
                  </td>
                </tr>
              </tfoot>
            </table>
          </div>
        </>
      )}
    </Dialog>
  )
}
