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
import type { OperatingRange } from './types'

/** 快捷区间，天数为 0 表示北京时间本月。 */
export const OPERATING_RANGE_PRESETS = [
  { labelKey: 'Current month', days: 0 },
  { labelKey: 'Last 7 days', days: 7 },
  { labelKey: 'Last 30 days', days: 30 },
  { labelKey: 'Last 90 days', days: 90 },
] as const

/** 发放来源的展示顺序与文案键。 */
export const OPERATING_GRANT_ORDER = [
  'paid',
  'signup',
  'invite',
  'redemption',
  'checkin',
  'admin',
] as const

export const OPERATING_GRANT_LABEL_KEYS: Record<string, string> = {
  paid: 'Paid top-up',
  signup: 'New user bonus',
  invite: 'Invitation bonus',
  redemption: 'Redemption code',
  checkin: 'Daily check-in',
  admin: 'Admin adjustment',
}

export const OPERATING_EXPENSE_CATEGORIES = [
  { value: 'upstream_package', labelKey: 'Upstream package' },
  { value: 'upstream_balance', labelKey: 'Upstream balance' },
  { value: 'server', labelKey: 'Server' },
  { value: 'other', labelKey: 'Other' },
] as const

export const OPERATING_EXPENSE_CATEGORY_LABEL_KEYS: Record<string, string> =
  Object.fromEntries(
    OPERATING_EXPENSE_CATEGORIES.map((item) => [item.value, item.labelKey])
  )

/**
 * 计算北京时间下自然月的起止。区间左闭右开，避免相邻月份重复计入月末当天。
 */
function beijingMonthRange(now: Date): OperatingRange {
  const offsetMs = 8 * 60 * 60 * 1000
  const beijingNow = new Date(now.getTime() + offsetMs)
  const year = beijingNow.getUTCFullYear()
  const month = beijingNow.getUTCMonth()

  const startAt = Date.UTC(year, month, 1) - offsetMs
  const endAt = Date.UTC(year, month + 1, 1) - offsetMs
  return {
    startTimestamp: Math.floor(startAt / 1000),
    endTimestamp: Math.floor(endAt / 1000),
  }
}

/** 按快捷预设计算区间；days 为 0 时取本月，其余取含今天在内最近 N 天。 */
export function buildOperatingRange(days: number, now = new Date()): OperatingRange {
  if (days === 0) {
    return beijingMonthRange(now)
  }
  const offsetMs = 8 * 60 * 60 * 1000
  const beijingNow = new Date(now.getTime() + offsetMs)
  const endAt = Date.UTC(
    beijingNow.getUTCFullYear(),
    beijingNow.getUTCMonth(),
    beijingNow.getUTCDate() + 1
  )
  const startAt = endAt - days * 24 * 60 * 60 * 1000
  return {
    startTimestamp: Math.floor((startAt - offsetMs) / 1000),
    endTimestamp: Math.floor((endAt - offsetMs) / 1000),
  }
}
