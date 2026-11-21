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
import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { OperatingSummaryCards } from '@/features/operating/components/operating-cards'
import type { OperatingRange, OperatingSummary } from '@/features/operating/types'

const range: OperatingRange = {
  startTimestamp: 1_700_000_000,
  endTimestamp: 1_700_086_400,
}

const summary: OperatingSummary = {
  start_timestamp: range.startTimestamp,
  end_timestamp: range.endTimestamp,
  revenue: {
    orders: 3,
    amount_cents: 1_250,
    credited_quota: 6_250_000,
    mixed_currency: false,
  },
  cost: { entries: 1, amount_cents: 15_000 },
  consumption: { quota: 2_400_000, users: 7 },
  balance: { users: 9, quota: 8_000_000, snapshot_at: 1_700_090_000 },
  grants: [],
  logs_available: true,
}

describe('OperatingSummaryCards', () => {
  it('renders one bordered strip with a divided 5-column grid', () => {
    const { container } = render(
      <OperatingSummaryCards summary={summary} range={range} loading={false} />
    )

    const strip = container.firstElementChild as HTMLElement
    expect(strip.className).toContain('rounded-lg')
    expect(strip.className).toContain('border')
    // 数据面板的指标带用分隔线网格，不是独立卡片。
    expect(strip.className).not.toContain('gap-3')

    const grid = strip.firstElementChild as HTMLElement
    expect(grid.className).toContain('divide-x')
    expect(grid.className).toContain('lg:grid-cols-5')
  })

  it('keeps the negative cash inflow sign so an outflow is never read as income', () => {
    render(
      <OperatingSummaryCards
        summary={{ ...summary, cost: { entries: 1, amount_cents: 500_000 } }}
        range={range}
        loading={false}
      />
    )

    const inflow = screen.getByRole('group', { name: 'Cash inflow' })
    // 金额经 @/lib/currency 格式化，这里只断言符号语义：流出必须带负号。
    expect(inflow.textContent).toContain('-')
    expect(inflow.textContent).toContain('4,987')
  })

  it('formats a positive cash inflow without a negative sign', () => {
    // 收入 12.50，支出只有 0.50 -> 净流入 12.00。
    render(
      <OperatingSummaryCards
        summary={{ ...summary, cost: { entries: 1, amount_cents: 50 } }}
        range={range}
        loading={false}
      />
    )

    const inflow = screen.getByRole('group', { name: 'Cash inflow' })
    expect(inflow.textContent).toContain('12')
    expect(inflow.textContent).not.toContain('-')
  })

  it('marks the strip busy while loading and hides stale values', () => {
    render(<OperatingSummaryCards range={range} loading />)

    const inflow = screen.getByRole('group', { name: 'Cash inflow' })
    expect(inflow.getAttribute('aria-busy')).toBe('true')
    expect(inflow.textContent).not.toContain('--')
  })
})
