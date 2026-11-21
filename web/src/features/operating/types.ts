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
export type OperatingAccuracy = 'exact' | 'estimated'

/** 一段时间区间，后端按左闭右开处理。 */
export interface OperatingRange {
  startTimestamp: number
  endTimestamp: number
}

export interface OperatingRevenue {
  orders: number
  /** 统一换算到人民币分的实收合计。 */
  amount_cents: number
  /** 这些订单实际到账的额度合计。 */
  credited_quota: number
  /** 期间内是否混有不同记账币种的渠道，此时金额只是换算值，不宜直接记账。 */
  mixed_currency: boolean
}

export interface OperatingCost {
  entries: number
  amount_cents: number
}

export interface OperatingConsumption {
  quota: number
  users: number
}

export interface OperatingBalance {
  users: number
  quota: number
  /** 余额快照时点，避免把当前余额误读成期末余额。 */
  snapshot_at: number
}

export interface OperatingGrant {
  key: string
  count: number
  quota: number
  accuracy: OperatingAccuracy
}

export interface OperatingSummary {
  start_timestamp: number
  end_timestamp: number
  revenue: OperatingRevenue
  cost: OperatingCost
  consumption: OperatingConsumption
  balance: OperatingBalance
  grants: OperatingGrant[]
  /** 日志库不可用或记录被清理时，消费与发放不完整，不能当成零消费展示。 */
  logs_available: boolean
}

export interface OperatingExpense {
  id: number
  category: string
  amount_cents: number
  occurred_at: number
  note: string
  operator_id: number
  created_at: number
}

export interface OperatingExpensePayload {
  category: string
  amount_cents: number
  occurred_at: number
  note: string
}
