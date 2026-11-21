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
import { CalendarRange, Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { SegmentedControl } from '@/components/ui/segmented-control'
import dayjs from '@/lib/dayjs'

import { OPERATING_RANGE_PRESETS } from '../constants'
import type { OperatingRange } from '../types'

import { OperatingExpenseDialog } from './operating-expense-dialog'

interface OperatingHeaderProps {
  range: OperatingRange
  activePreset: number
  onPresetChange: (days: number) => void
}

/** 与数据面板顶部的分段筛选一致：浅底圆角容器里的分段按钮。 */
export function OperatingHeader(props: OperatingHeaderProps) {
  const { t } = useTranslation()
  const [expenseDialogOpen, setExpenseDialogOpen] = useState(false)

  const startLabel = dayjs(props.range.startTimestamp * 1000).format(
    'MM-DD HH:mm'
  )
  const endLabel = dayjs(props.range.endTimestamp * 1000 - 1000).format(
    'MM-DD HH:mm'
  )

  return (
    <>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <div className='flex flex-wrap items-center gap-2'>
          <SegmentedControl
            value={String(props.activePreset)}
            options={OPERATING_RANGE_PRESETS.map((preset) => ({
              value: String(preset.days),
              label: t(preset.labelKey),
            }))}
            onValueChange={(value) => props.onPresetChange(Number(value))}
          />
          <span className='text-muted-foreground flex items-center gap-1 text-xs tabular-nums'>
            <CalendarRange className='size-3.5' aria-hidden='true' />
            {startLabel} ~ {endLabel}
          </span>
        </div>
        <Button
          size='sm'
          variant='outline'
          className='h-8'
          onClick={() => setExpenseDialogOpen(true)}
        >
          <Plus className='size-4' aria-hidden='true' />
          {t('Record expense')}
        </Button>
      </div>

      <OperatingExpenseDialog
        open={expenseDialogOpen}
        onOpenChange={setExpenseDialogOpen}
      />
    </>
  )
}
