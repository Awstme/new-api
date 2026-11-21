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

import { useCreateOperatingRefund } from '../hooks/use-operating'
import { generateOperatingOperationId } from '../lib/operation-id'

interface OperatingRefundDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function OperatingRefundDialog(props: OperatingRefundDialogProps) {
  const { t } = useTranslation()
  const createMutation = useCreateOperatingRefund()
  const [amount, setAmount] = useState('')
  const [occurredAt, setOccurredAt] = useState<Date | undefined>(new Date())
  const [note, setNote] = useState('')
  // 同一次填写复用同一个 operation_id，网络重试不会产生第二笔退款。
  const [operationId, setOperationId] = useState(() =>
    generateOperatingOperationId()
  )

  const resetForm = () => {
    setAmount('')
    setOccurredAt(new Date())
    setNote('')
    setOperationId(generateOperatingOperationId())
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
        operation_id: operationId,
        amount_cents: cents,
        occurred_at: Math.floor(occurredAt.getTime() / 1000),
        note: note.trim(),
      },
      {
        onSuccess: () => {
          toast.success(t('Refund recorded'))
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
      title={t('Record refund')}
      description={t(
        'Records cash actually refunded. It does not change the user balance, because the wallet cannot tell paid credit from free credit yet.'
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
            form='operating-refund-form'
            disabled={createMutation.isPending}
          >
            {t('Save')}
          </Button>
        </>
      }
    >
      <form
        id='operating-refund-form'
        className='space-y-4'
        onSubmit={handleSubmit}
      >
        <div className='space-y-2'>
          <Label htmlFor='operating-refund-amount'>{t('Amount')}</Label>
          <Input
            id='operating-refund-amount'
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
          <Label htmlFor='operating-refund-note'>{t('Note')}</Label>
          <Input
            id='operating-refund-note'
            value={note}
            onChange={(event) => setNote(event.target.value)}
            maxLength={200}
          />
        </div>
      </form>
    </Dialog>
  )
}
