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
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'
import { useAffiliateRewards } from '../../hooks/use-affiliate-rewards'
import { formatTimestamp } from '../../lib/billing'
import type {
  AffiliateRewardRecord,
  AffiliateRewardStatus,
  AffiliateRewardTriggerType,
} from '../../types'

interface AffiliateRewardHistoryDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const rewardStatusConfig: Record<
  AffiliateRewardStatus,
  { label: string; variant: StatusVariant }
> = {
  pending: { label: 'Pending', variant: 'warning' },
  available: { label: 'Available', variant: 'success' },
  transferred: { label: 'Transferred', variant: 'neutral' },
  voided: { label: 'Voided', variant: 'danger' },
}

const triggerLabels: Record<AffiliateRewardTriggerType, string> = {
  first_topup: 'First top-up',
  recurring_topup: 'Recurring top-up',
  subscription_order: 'Subscription order',
}

function formatRate(rate: number) {
  const percent = (rate * 100).toFixed(2)
  return `${percent.replace(/\.?0+$/, '')}%`
}

function formatTimeOrDash(timestamp: number) {
  return timestamp > 0 ? formatTimestamp(timestamp) : '-'
}

function RewardRecordItem({ record }: { record: AffiliateRewardRecord }) {
  const { t } = useTranslation()
  const statusConfig =
    rewardStatusConfig[record.status] || rewardStatusConfig.pending
  const triggerLabel = triggerLabels[record.trigger_type] || record.trigger_type

  return (
    <div className='hover:bg-muted/50 rounded-lg border p-3 transition-colors sm:p-4'>
      <div className='flex items-start justify-between gap-2'>
        <div className='min-w-0 space-y-1'>
          <div className='truncate text-sm font-medium'>{t(triggerLabel)}</div>
          <code className='text-muted-foreground block truncate font-mono text-xs'>
            {record.trade_no}
          </code>
        </div>
        <StatusBadge
          label={t(statusConfig.label)}
          variant={statusConfig.variant}
          showDot
          copyable={false}
        />
      </div>

      <div className='mt-3 grid grid-cols-2 gap-3 sm:mt-4 sm:grid-cols-4 sm:gap-4'>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Top-up quota')}
          </Label>
          <div className='text-sm font-semibold'>
            {formatQuota(record.basis_quota)}
          </div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Reward Rate')}
          </Label>
          <div className='text-sm font-semibold'>
            {formatRate(record.reward_rate)}
          </div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Reward quota')}
          </Label>
          <div className='text-success text-sm font-semibold'>
            {formatQuota(record.reward_quota)}
          </div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Available At')}
          </Label>
          <div className='text-sm font-medium'>
            {formatTimeOrDash(record.eligible_at)}
          </div>
        </div>
      </div>

      <div className='text-muted-foreground mt-3 text-xs'>
        {t('Created')}: {formatTimeOrDash(record.created_at)}
      </div>
      {record.status === 'voided' && record.void_reason ? (
        <div className='text-destructive mt-2 text-xs'>
          {t('Void reason')}: {record.void_reason}
        </div>
      ) : null}
    </div>
  )
}

export function AffiliateRewardHistoryDialog({
  open,
  onOpenChange,
}: AffiliateRewardHistoryDialogProps) {
  const { t } = useTranslation()
  const {
    records,
    total,
    page,
    pageSize,
    loading,
    handlePageChange,
    handlePageSizeChange,
  } = useAffiliateRewards({ enabled: open })

  const totalPages = Math.max(1, Math.ceil(total / pageSize))

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>{t('Reward History')}</DialogTitle>
          <DialogDescription>
            {t('View pending, available, transferred, and voided rewards.')}
          </DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-3 sm:space-y-4'>
          <div className='flex items-center justify-end'>
            <Select
              items={[
                { value: '10', label: t('10 / page') },
                { value: '20', label: t('20 / page') },
                { value: '50', label: t('50 / page') },
                { value: '100', label: t('100 / page') },
              ]}
              value={pageSize.toString()}
              onValueChange={(value) =>
                value !== null && handlePageSizeChange(parseInt(value))
              }
            >
              <SelectTrigger className='h-9 w-[92px] sm:w-32'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  <SelectItem value='10'>{t('10 / page')}</SelectItem>
                  <SelectItem value='20'>{t('20 / page')}</SelectItem>
                  <SelectItem value='50'>{t('50 / page')}</SelectItem>
                  <SelectItem value='100'>{t('100 / page')}</SelectItem>
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <ScrollArea className='h-[calc(100dvh-13rem)] pr-3 sm:h-[500px] sm:pr-4'>
            {loading ? (
              <div className='space-y-3'>
                {Array.from({ length: 5 }).map((_, index) => (
                  <div key={index} className='rounded-lg border p-3 sm:p-4'>
                    <div className='flex items-start justify-between'>
                      <div className='flex-1 space-y-2'>
                        <Skeleton className='h-4 w-40' />
                        <Skeleton className='h-3 w-56' />
                      </div>
                      <Skeleton className='h-5 w-20' />
                    </div>
                    <div className='mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4'>
                      <Skeleton className='h-3 w-full' />
                      <Skeleton className='h-3 w-full' />
                      <Skeleton className='h-3 w-full' />
                      <Skeleton className='h-3 w-full' />
                    </div>
                  </div>
                ))}
              </div>
            ) : records.length === 0 ? (
              <div className='text-muted-foreground flex h-[320px] flex-col items-center justify-center text-center sm:h-[400px]'>
                <p className='text-sm font-medium'>
                  {t('No reward records found')}
                </p>
                <p className='mt-1 text-xs'>
                  {t('Reward records will appear here after eligible top-ups.')}
                </p>
              </div>
            ) : (
              <div className='space-y-3'>
                {records.map((record) => (
                  <RewardRecordItem key={record.id} record={record} />
                ))}
              </div>
            )}
          </ScrollArea>

          {!loading && records.length > 0 && (
            <div className='flex flex-col items-center gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
              <div className='text-muted-foreground text-xs sm:text-sm'>
                {t('Showing')} {(page - 1) * pageSize + 1}-
                {Math.min(page * pageSize, total)} {t('of')} {total}
              </div>
              <div className='flex items-center gap-2'>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => handlePageChange(page - 1)}
                  disabled={page <= 1}
                  className='h-8 w-8 p-0'
                >
                  <ChevronLeft data-icon='standalone' />
                </Button>
                <div className='text-muted-foreground flex items-center gap-1 text-sm'>
                  <span className='font-medium'>{page}</span>
                  <span>/</span>
                  <span>{totalPages}</span>
                </div>
                <Button
                  variant='outline'
                  size='sm'
                  onClick={() => handlePageChange(page + 1)}
                  disabled={page >= totalPages}
                  className='h-8 w-8 p-0'
                >
                  <ChevronRight data-icon='standalone' />
                </Button>
              </div>
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  )
}
