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
import { ChevronDown, ChevronRight, Copy } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatQuota, formatTimestamp } from '@/lib/format'

import type {
  FreeQuotaSource,
  FreeQuotaStats,
  FreeQuotaUser,
} from '../../types'
import { FreeQuotaMethodBadge } from './free-quota-badge'
import {
  FREE_QUOTA_METHOD_LABELS,
  FREE_QUOTA_SOURCE_LABELS,
} from './free-quota-labels'

interface FreeQuotaDetailsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  stats?: FreeQuotaStats
  timeRange?: {
    start_timestamp: number
    end_timestamp: number
  }
}

function userKey(source: string, userId: number) {
  return `${source}:${userId}`
}

function copyText(text: string) {
  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand('copy')
  textarea.remove()
  return copied
}

export function FreeQuotaDetailsDialog(props: FreeQuotaDetailsDialogProps) {
  const { t } = useTranslation()
  const visibleSources = useMemo(
    () =>
      (props.stats?.sources ?? []).filter(
        (source) => source.count > 0 && source.quota > 0
      ),
    [props.stats]
  )
  const allUsers = useMemo(
    () =>
      visibleSources.flatMap((source) =>
        source.users.map((user) => ({ source, user }))
      ),
    [visibleSources]
  )
  const allUserKeys = useMemo(
    () => allUsers.map(({ source, user }) => userKey(source.key, user.user_id)),
    [allUsers]
  )
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  useEffect(() => {
    if (props.open) {
      setSelected(new Set(allUserKeys))
    }
  }, [props.open, allUserKeys])

  useEffect(() => {
    if (!props.open) {
      setExpanded(new Set())
    }
  }, [props.open])

  const selectedTotal = allUsers.reduce(
    (total, { source, user }) =>
      selected.has(userKey(source.key, user.user_id)) ? total + user.used : total,
    0
  )
  const visibleUsedTotal = visibleSources.reduce(
    (total, source) => total + source.quota,
    0
  )
  const visibleGrantedTotal = visibleSources.reduce(
    (total, source) => total + source.granted,
    0
  )
  const allSelected =
    allUserKeys.length > 0 && selected.size === allUserKeys.length
  const someSelected = selected.size > 0 && !allSelected
  const allExpanded =
    visibleSources.length > 0 &&
    visibleSources.every((source) => expanded.has(source.key))

  const toggleUsers = (keys: string[], checked: boolean) => {
    setSelected((previous) => {
      const next = new Set(previous)
      for (const key of keys) {
        if (checked) next.add(key)
        else next.delete(key)
      }
      return next
    })
  }

  const toggleExpanded = (source: string) => {
    setExpanded((previous) => {
      const next = new Set(previous)
      if (next.has(source)) next.delete(source)
      else next.add(source)
      return next
    })
  }

  const toggleAllExpanded = () => {
    setExpanded(
      allExpanded
        ? new Set()
        : new Set(visibleSources.map((source) => source.key))
    )
  }

  const copySelected = async () => {
    const header = [
      t('Source'),
      t('User'),
      t('Used'),
      t('Granted'),
      t('Grant count'),
      t('How it was obtained'),
    ].join('\t')
    const rows = allUsers
      .filter(({ source, user }) =>
        selected.has(userKey(source.key, user.user_id))
      )
      .map(({ source, user }) =>
        [
          t(FREE_QUOTA_SOURCE_LABELS[source.key] ?? source.key),
          user.username,
          formatQuota(user.used),
          source.kind === 'paid' ? '—' : formatQuota(user.granted),
          source.kind === 'paid' ? '—' : String(user.grant_count),
          t(FREE_QUOTA_METHOD_LABELS[user.method] ?? user.method),
        ].join('\t')
      )
    const text = [header, ...rows].join('\n')
    try {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text)
        toast.success(t('Copied to clipboard'))
        return
      }
      if (copyText(text)) {
        toast.success(t('Copied to clipboard'))
        return
      }
      toast.error(t('Failed to copy'))
    } catch {
      if (copyText(text)) {
        toast.success(t('Copied to clipboard'))
      } else {
        toast.error(t('Failed to copy'))
      }
    }
  }

  let body
  if (visibleSources.length === 0) {
    body = (
      <div className='text-muted-foreground py-8 text-center text-xs'>
        {t('No free quota was consumed in this range')}
      </div>
    )
  } else {
    body = (
      <>
        <div className='flex flex-wrap items-center justify-between gap-2'>
          <div className='text-muted-foreground text-xs'>
            {selected.size} {t('Selected')}
            <span className='mx-1'>·</span>
            {t('Used')} {formatQuota(selectedTotal)}
          </div>
          <div className='flex items-center gap-1'>
            <Button
              variant='ghost'
              size='sm'
              className='text-muted-foreground'
              onClick={toggleAllExpanded}
            >
              {allExpanded ? t('Collapse all') : t('Expand all')}
            </Button>
            <Button
              variant='outline'
              size='sm'
              disabled={selected.size === 0}
              onClick={() => void copySelected()}
            >
              <Copy className='size-3.5' />
              {t('Copy selected')}
            </Button>
          </div>
        </div>

        <div className='overflow-x-auto rounded-md border'>
          <table className='w-full min-w-[760px] text-xs'>
            <thead className='bg-muted/40 text-muted-foreground text-left'>
              <tr>
                <th className='w-8 px-3 py-2'>
                  <Checkbox
                    checked={allSelected}
                    indeterminate={someSelected}
                    aria-label={t('Select all')}
                    onCheckedChange={(checked) =>
                      toggleUsers(allUserKeys, checked === true)
                    }
                  />
                </th>
                <th className='w-8 px-1 py-2' />
                <th className='px-3 py-2 font-medium'>{t('Source')}</th>
                <th className='px-3 py-2 text-right font-medium'>
                  {t('Used')}
                </th>
                <th className='px-3 py-2 text-right font-medium'>
                  {t('Granted')}
                </th>
                <th className='px-3 py-2 text-right font-medium'>
                  {t('Count')}
                </th>
                <th className='px-3 py-2 font-medium'>
                  {t('How it was obtained')}
                </th>
              </tr>
            </thead>
            <tbody>
              {visibleSources.map((source) => (
                <FreeQuotaSourceRows
                  key={source.key}
                  source={source}
                  selected={selected}
                  expanded={expanded.has(source.key)}
                  timeRange={props.timeRange}
                  onToggleUsers={toggleUsers}
                  onToggleExpanded={() => toggleExpanded(source.key)}
                />
              ))}
            </tbody>
            <tfoot className='bg-muted/30'>
              <tr className='border-border/60 border-t'>
                <td className='px-3 py-2' />
                <td className='px-1 py-2' />
                <td className='px-3 py-2 font-semibold'>{t('Total')}</td>
                <td className='px-3 py-2 text-right font-mono font-semibold tabular-nums'>
                  {formatQuota(visibleUsedTotal)}
                </td>
                <td className='px-3 py-2 text-right font-mono font-semibold tabular-nums'>
                  {formatQuota(visibleGrantedTotal)}
                </td>
                <td className='px-3 py-2' />
                <td className='px-3 py-2' />
              </tr>
            </tfoot>
          </table>
        </div>
        <div className='text-muted-foreground space-y-1 text-[11px] leading-relaxed'>
          <p>
            {t(
              'Only actually consumed free quota is counted; unused grants are excluded. Consumption is attributed to grants oldest-first.'
            )}
          </p>
          <p className='text-muted-foreground/70'>
            {t(
              'Used quota can come from grants made before the selected range; Granted only counts grants made inside the range.'
            )}
          </p>
          <p className='text-muted-foreground/70'>
            {t(
              'Normal quota is total consumption minus consumed free quota.'
            )}
          </p>
        </div>
      </>
    )
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Quota consumption details')}
      description={t(
        'Actual consumption by source and how each amount was obtained.'
      )}
      contentClassName='sm:max-w-4xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
    >
      {body}
    </Dialog>
  )
}

interface FreeQuotaSourceRowsProps {
  source: FreeQuotaSource
  selected: Set<string>
  expanded: boolean
  timeRange?: {
    start_timestamp: number
    end_timestamp: number
  }
  onToggleUsers: (keys: string[], checked: boolean) => void
  onToggleExpanded: () => void
}

function FreeQuotaSourceRows(props: FreeQuotaSourceRowsProps) {
  const { t } = useTranslation()
  const userKeys = props.source.users.map((user) =>
    userKey(props.source.key, user.user_id)
  )
  const selectedCount = userKeys.filter((key) =>
    props.selected.has(key)
  ).length
  const allSelected = userKeys.length > 0 && selectedCount === userKeys.length
  const someSelected = selectedCount > 0 && !allSelected

  return (
    <>
      <tr className='border-border/60 bg-muted/20 border-t'>
        <td className='px-3 py-2'>
          <Checkbox
            checked={allSelected}
            indeterminate={someSelected}
            aria-label={t(FREE_QUOTA_SOURCE_LABELS[props.source.key] ?? props.source.key)}
            onCheckedChange={(checked) =>
              props.onToggleUsers(userKeys, checked === true)
            }
          />
        </td>
        <td className='px-1 py-2'>
          <Button
            variant='ghost'
            size='icon-xs'
            aria-label={props.expanded ? t('Collapse') : t('Expand')}
            onClick={props.onToggleExpanded}
          >
            {props.expanded ? (
              <ChevronDown className='size-3.5' />
            ) : (
              <ChevronRight className='size-3.5' />
            )}
          </Button>
        </td>
        <td className='px-3 py-2 font-medium'>
          {t(FREE_QUOTA_SOURCE_LABELS[props.source.key] ?? props.source.key)}
        </td>
        <td className='px-3 py-2 text-right font-mono tabular-nums'>
          {formatQuota(props.source.quota)}
        </td>
        <td className='text-muted-foreground px-3 py-2 text-right font-mono tabular-nums'>
          {props.source.kind === 'paid'
            ? '—'
            : formatQuota(props.source.granted)}
        </td>
        <td className='text-muted-foreground px-3 py-2 text-right'>
          {props.source.count} {t('Users')}
        </td>
        <td className='px-3 py-2'>
          <FreeQuotaMethodBadge method={props.source.method} />
        </td>
      </tr>
      {props.expanded &&
        props.source.users.map((user) => (
          <FreeQuotaUserRow
            key={user.user_id}
            source={props.source.key}
            user={user}
            paid={props.source.kind === 'paid'}
            selected={props.selected.has(
              userKey(props.source.key, user.user_id)
            )}
            timeRange={props.timeRange}
            onToggle={(checked) =>
              props.onToggleUsers(
                [userKey(props.source.key, user.user_id)],
                checked
              )
            }
          />
        ))}
    </>
  )
}

interface FreeQuotaUserRowProps {
  source: string
  user: FreeQuotaUser
  paid: boolean
  selected: boolean
  timeRange?: {
    start_timestamp: number
    end_timestamp: number
  }
  onToggle: (checked: boolean) => void
}

function FreeQuotaUserRow(props: FreeQuotaUserRowProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const openUserLogs = () => {
    void navigate({
      to: '/usage-logs/$section',
      params: { section: 'common' },
      search: {
        username: props.user.username,
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
  return (
    <tr className='border-border/60 border-t'>
      <td className='px-3 py-2'>
        <Checkbox
          checked={props.selected}
          aria-label={props.user.username}
          onCheckedChange={(checked) => props.onToggle(checked === true)}
        />
      </td>
      <td className='px-1 py-2' />
      <td className='px-3 py-2'>
        <button
          type='button'
          title={t("View this user's logs")}
          className='flex min-w-0 items-center gap-2 text-left'
          onClick={openUserLogs}
        >
          <Avatar className='ring-border/60 size-6 ring-1'>
            <AvatarFallback
              className='text-[11px] font-semibold'
              style={getUserAvatarStyle(props.user.username)}
            >
              {getUserAvatarFallback(props.user.username)}
            </AvatarFallback>
          </Avatar>
          <span className='truncate font-medium hover:underline'>
            {props.user.username}
          </span>
        </button>
        {props.user.last_used_at > 0 && (
          <div className='text-muted-foreground/70 mt-0.5 text-[10px]'>
            {t('Last used')} {formatTimestamp(props.user.last_used_at)}
          </div>
        )}
      </td>
      <td className='px-3 py-2 text-right font-mono tabular-nums'>
        {formatQuota(props.user.used)}
      </td>
      <td className='text-muted-foreground px-3 py-2 text-right font-mono tabular-nums'>
        {props.paid ? '—' : formatQuota(props.user.granted)}
      </td>
      <td className='text-muted-foreground px-3 py-2 text-right'>
        {props.paid ? '—' : `${props.user.grant_count} ${t('grants')}`}
      </td>
      <td className='px-3 py-2'>
        <FreeQuotaMethodBadge method={props.user.method} />
      </td>
    </tr>
  )
}
