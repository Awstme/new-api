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
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { FREE_QUOTA_METHOD_LABELS } from './free-quota-labels'

export function FreeQuotaMethodBadge(props: { method: string }) {
  const { t } = useTranslation()
  let className = ''
  if (props.method === 'exact') {
    className =
      'bg-emerald-100 text-emerald-700 dark:bg-emerald-950/40 dark:text-emerald-400'
  } else if (props.method === 'log_text') {
    className = 'bg-sky-100 text-sky-700 dark:bg-sky-950/40 dark:text-sky-400'
  } else if (props.method === 'config') {
    className =
      'bg-amber-100 text-amber-700 dark:bg-amber-950/40 dark:text-amber-400'
  } else if (props.method === 'mixed') {
    className =
      'bg-violet-100 text-violet-700 dark:bg-violet-950/40 dark:text-violet-400'
  }
  const label = FREE_QUOTA_METHOD_LABELS[props.method]
  if (!label) return null
  return (
    <span
      className={cn(
        'inline-flex rounded px-1 text-[9px] font-medium',
        className
      )}
    >
      {t(label)}
    </span>
  )
}