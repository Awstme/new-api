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
import { describe, expect, it } from 'vitest'

import {
  beijingDayKey,
  buildOperatingRange,
  OPERATING_DEFAULT_RANGE_DAYS,
  OPERATING_RANGE_PRESETS,
} from '@/features/operating/constants'

const BEIJING_OFFSET_MS = 8 * 60 * 60 * 1000

/** 把秒级区间端点还原成北京时间的日期串，便于断言自然日边界。 */
function beijingDay(timestamp: number): string {
  return new Date(timestamp * 1000 + BEIJING_OFFSET_MS)
    .toISOString()
    .slice(0, 10)
}

describe('OPERATING_RANGE_PRESETS', () => {
  it('offers Today first and uses it as the default range', () => {
    expect(OPERATING_RANGE_PRESETS[0]).toEqual({ labelKey: 'Today', days: 1 })
    expect(OPERATING_DEFAULT_RANGE_DAYS).toBe(
      OPERATING_RANGE_PRESETS[0].days
    )
  })
})

describe('buildOperatingRange', () => {
  // 北京时间 12:15，尚未跨日；区间应是当天 0 点到次日 0 点。
  const now = new Date('2026-11-21T12:15:00+08:00')

  it('covers the current Beijing day for the Today preset', () => {
    const range = buildOperatingRange(1, now)

    expect(beijingDay(range.startTimestamp)).toBe('2026-11-21')
    expect(beijingDay(range.endTimestamp - 1)).toBe('2026-11-21')
    // 左闭右开：结束点正好是次日零点。
    expect(range.endTimestamp - range.startTimestamp).toBe(24 * 60 * 60)
  })

  it('covers the whole Beijing month for the current-month preset', () => {
    const range = buildOperatingRange(0, now)

    expect(beijingDay(range.startTimestamp)).toBe('2026-11-01')
    expect(beijingDay(range.endTimestamp)).toBe('2026-12-01')
  })

  it('counts the trailing days including today', () => {
    const range = buildOperatingRange(7, now)

    expect(beijingDay(range.startTimestamp)).toBe('2026-11-15')
    expect(beijingDay(range.endTimestamp - 1)).toBe('2026-11-21')
  })

  it('starts the day at Beijing midnight, not UTC midnight', () => {
    const range = buildOperatingRange(1, now)

    // 北京时间零点 = 前一日 UTC 16:00。
    expect(range.startTimestamp).toBe(Date.UTC(2026, 10, 20, 16, 0, 0) / 1000)
  })
})

describe('beijingDayKey', () => {
  it('rolls over at Beijing midnight rather than UTC midnight', () => {
    // 北京时间 00:00:01 已是次日；UTC 还是前一天 16:00。
    expect(beijingDayKey(new Date('2026-11-21T00:00:01+08:00'))).toBe(
      '2026-11-21'
    )
    // 北京时间 23:59:59 仍属于当天。
    expect(beijingDayKey(new Date('2026-11-21T23:59:59+08:00'))).toBe(
      '2026-11-21'
    )
  })
})
