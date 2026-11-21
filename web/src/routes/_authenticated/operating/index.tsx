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
import { createFileRoute, redirect } from '@tanstack/react-router'

import { OPERATING_DEFAULT_SECTION } from '@/features/operating/section-registry'

/**
 * `/operating` 没有 section 段，重定向到默认区块，让 tab 与 URL 保持一致。
 */
export const Route = createFileRoute('/_authenticated/operating/')({
  beforeLoad: () => {
    throw redirect({
      to: '/operating/$section',
      params: { section: OPERATING_DEFAULT_SECTION },
    })
  },
})
