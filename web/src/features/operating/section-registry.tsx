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
import { createSectionRegistry } from '@/features/system-settings/utils/section-registry'

/**
 * 经营分析的页内区块：概览与流水日志各占一个 tab。
 */
const OPERATING_SECTIONS = [
  {
    id: 'overview',
    titleKey: 'Overview',
    build: () => null,
  },
  {
    id: 'flow',
    titleKey: 'Flow records',
    build: () => null,
  },
] as const

export type OperatingSectionId = (typeof OPERATING_SECTIONS)[number]['id']

const operatingRegistry = createSectionRegistry<
  OperatingSectionId,
  Record<string, never>,
  []
>({
  sections: OPERATING_SECTIONS,
  defaultSection: 'overview',
  basePath: '/operating',
  urlStyle: 'path',
})

export const OPERATING_SECTION_IDS = operatingRegistry.sectionIds
export const OPERATING_DEFAULT_SECTION = operatingRegistry.defaultSection
