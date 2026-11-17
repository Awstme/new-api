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
import { LOG_TYPE_ENUM } from '../constants'
import { parseLogOther } from './format'

export type CommonLogRowTone = 'error' | 'refund' | 'saturation' | null

export function getCommonLogRowTone(
  log: { type?: number; other?: string },
  isAdmin: boolean
): CommonLogRowTone {
  const other = parseLogOther(log.other ?? '')
  if (isAdmin && other?.admin_info?.quota_saturation) {
    return 'saturation'
  }
  if (
    log.type === LOG_TYPE_ENUM.ERROR ||
    (other?.session_error_count ?? 0) > 0
  ) {
    return 'error'
  }
  if (log.type === LOG_TYPE_ENUM.REFUND) {
    return 'refund'
  }
  return null
}
