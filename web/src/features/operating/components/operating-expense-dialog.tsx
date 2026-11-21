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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { OPERATING_EXPENSE_CATEGORIES } from '../constants'
import { useCreateOperatingExpense } from '../hooks/use-operating'

interface OperatingExpenseDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function OperatingExpenseDialog(props: OperatingExpenseDialogProps) {
  const { t } = useTranslation()
  const createMutation = useCreateOperatingExpense()
  const [category, setCategory] = useState<string>(
    OPERATING_EXPENSE_CATEGORIES[0].value
  )
  const [amount, setAmount] = useState('')
  const [occurredAt, setOccurredAt] = useState<Date | undefined>(new Date())
  const [note, setNote] = useState('')

  const resetForm = () => {
    setCategory(OPERATING_EXPENSE_CATEGORIES[0].value)
    setAmount('')
    setOccurredAt(new Date())
    setNote('')
  }

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const cents = Math.round(Number(amount) * 100)
    if (!Number.isFinite(cents) || cents <= 0) {
      toast.error(t('Amount must be greater than zero'))
      return
    }
    if (!occurredAt) {
      toast.error(t('Date is required'))
      return
    }
    createMutation.mutate(
      {
        category,
        amount_cents: cents,
        occurred_at: Math.floor(occurredAt.getTime() / 1000),
        note: note.trim(),
      },
      {
        onSuccess: () => {
          toast.success(t('Expense recorded'))
          resetForm()
          props.onOpenChange(false)
        },
      }
    )
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) resetForm()
        props.onOpenChange(open)
      }}
      title={t('Record expense')}
      description={t(
        'Record what was actually paid. The amount is counted in the period it occurred, not when it was entered.'
      )}
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='operating-expense-form'
            disabled={createMutation.isPending}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      <form
        id='operating-expense-form'
        className='space-y-4'
        onSubmit={handleSubmit}
      >
        <div className='space-y-2'>
          <Label htmlFor='operating-expense-category'>{t('Category')}</Label>
          <Select value={category} onValueChange={(value) => setCategory(value ?? '')}>
            <SelectTrigger id='operating-expense-category'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {OPERATING_EXPENSE_CATEGORIES.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {t(item.labelKey)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='operating-expense-amount'>{t('Amount')}</Label>
          <Input
            id='operating-expense-amount'
            inputMode='decimal'
            value={amount}
            onChange={(event) => setAmount(event.target.value)}
            placeholder='0.00'
          />
        </div>
        <div className='space-y-2'>
          <Label>{t('Date')}</Label>
          <DateTimePicker value={occurredAt} onChange={setOccurredAt} />
        </div>
        <div className='space-y-2'>
          <Label htmlFor='operating-expense-note'>{t('Note')}</Label>
          <Input
            id='operating-expense-note'
            value={note}
            onChange={(event) => setNote(event.target.value)}
            maxLength={200}
          />
        </div>
      </form>
    </Dialog>
  )
}
