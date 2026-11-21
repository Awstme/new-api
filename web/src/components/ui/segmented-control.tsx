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
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export interface SegmentedControlOption<T extends string> {
  value: T
  label: ReactNode
  icon?: LucideIcon
}

interface SegmentedControlProps<T extends string> {
  value: T
  options: readonly SegmentedControlOption<T>[]
  onValueChange: (value: T) => void
  className?: string
}

/**
 * 紧凑分段选择器：浅底容器内的互斥按钮。
 *
 * 与 `Tabs` 的区别是视觉与语义：`Tabs` 切换的是页面区块，这里切换的是同一份
 * 数据的不同视图，因此用 `aria-pressed` 而不是 tablist/tab 语义。
 */
export function SegmentedControl<T extends string>(props: SegmentedControlProps<T>) {
  return (
    <div
      className={cn(
        'bg-muted/60 inline-flex h-8 overflow-x-auto rounded-lg border p-0.5',
        props.className
      )}
    >
      {props.options.map((option) => {
        const Icon = option.icon
        const selected = option.value === props.value
        return (
          <button
            key={option.value}
            type='button'
            aria-pressed={selected}
            onClick={() => props.onValueChange(option.value)}
            className={cn(
              'inline-flex shrink-0 items-center gap-1.5 rounded-md px-3 text-xs font-medium transition-colors',
              selected
                ? 'bg-background text-foreground shadow-sm'
                : 'text-muted-foreground hover:text-foreground'
            )}
          >
            {Icon != null && <Icon className='size-3.5' aria-hidden='true' />}
            {option.label}
          </button>
        )
      })}
    </div>
  )
}
