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
import { Ban, ChevronLeft, ChevronRight, Search } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatQuota } from '@/lib/format'
import { useIsAdmin } from '@/hooks/use-admin'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
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
  AffiliateRewardSourceType,
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

const sourceLabels: Record<AffiliateRewardSourceType, string> = {
  topup: 'Top-up order',
  subscription_order: 'Subscription order',
}

const paymentProviderLabels: Record<string, string> = {
  epay: 'Epay',
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  waffo_pancake: 'Waffo Pancake',
}

function formatRate(rate: number) {
  const percent = (rate * 100).toFixed(2)
  return `${percent.replace(/\.?0+$/, '')}%`
}

function formatTimeOrDash(timestamp: number) {
  return timestamp > 0 ? formatTimestamp(timestamp) : '-'
}

function RewardRecordItem({
  record,
  isAdmin,
  onVoid,
  voiding,
}: {
  record: AffiliateRewardRecord
  isAdmin: boolean
  onVoid: (record: AffiliateRewardRecord) => void
  voiding: boolean
}) {
  const { t } = useTranslation()
  const statusConfig =
    rewardStatusConfig[record.status] || rewardStatusConfig.pending
  const triggerLabel = triggerLabels[record.trigger_type] || record.trigger_type
  const sourceLabel = record.source_type
    ? sourceLabels[record.source_type] || record.source_type
    : '-'
  const providerLabel = record.payment_provider
    ? paymentProviderLabels[record.payment_provider] || record.payment_provider
    : '-'
  const remainingQuota = Math.max(
    0,
    record.reward_quota - (record.transferred_quota ?? 0)
  )

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
      {isAdmin ? (
        <div className='mt-2 flex flex-wrap gap-2'>
          <StatusBadge
            label={`${t('Inviter ID')}: ${record.inviter_id}`}
            variant='neutral'
            size='sm'
            copyText={String(record.inviter_id)}
          />
          <StatusBadge
            label={`${t('Invitee ID')}: ${record.invitee_id}`}
            variant='neutral'
            size='sm'
            copyText={String(record.invitee_id)}
          />
        </div>
      ) : null}

      <div className='mt-3 grid grid-cols-2 gap-3 sm:mt-4 sm:grid-cols-3 sm:gap-4 lg:grid-cols-4'>
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
            {t('Remaining quota')}
          </Label>
          <div
            className={
              remainingQuota > 0
                ? 'text-success text-sm font-semibold'
                : 'text-muted-foreground text-sm font-semibold'
            }
          >
            {formatQuota(remainingQuota)}
          </div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>{t('Source')}</Label>
          <div className='text-sm font-medium'>{t(sourceLabel)}</div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Payment provider')}
          </Label>
          <div className='text-sm font-medium'>{t(providerLabel)}</div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Available At')}
          </Label>
          <div className='text-sm font-medium'>
            {formatTimeOrDash(record.eligible_at)}
          </div>
        </div>
        <div className='space-y-1'>
          <Label className='text-muted-foreground text-xs'>
            {t('Transferred At')}
          </Label>
          <div className='text-sm font-medium'>
            {formatTimeOrDash(record.transferred_at ?? 0)}
          </div>
        </div>
      </div>

      <div className='mt-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='text-muted-foreground text-xs'>
          {t('Created')}: {formatTimeOrDash(record.created_at)}
        </div>
        {isAdmin && record.status !== 'voided' ? (
          <Button
            size='sm'
            variant='outline'
            className='h-8 w-fit'
            disabled={voiding}
            onClick={() => onVoid(record)}
          >
            <Ban data-icon='inline-start' />
            {t('Void reward')}
          </Button>
        ) : null}
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
  const isAdmin = useIsAdmin()
  const [voidTarget, setVoidTarget] = useState<AffiliateRewardRecord | null>(
    null
  )
  const [voidReason, setVoidReason] = useState('')
  const {
    records,
    total,
    summary,
    page,
    pageSize,
    keyword,
    status,
    triggerType,
    sourceType,
    paymentProvider,
    startTime,
    endTime,
    loading,
    voiding,
    handlePageChange,
    handlePageSizeChange,
    handleSearch,
    handleStatusChange,
    handleTriggerTypeChange,
    handleSourceTypeChange,
    handlePaymentProviderChange,
    handleTimeRangeChange,
    handleVoidReward,
  } = useAffiliateRewards({ enabled: open, admin: isAdmin })

  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const handleConfirmVoid = async () => {
    if (!voidTarget) return
    const success = await handleVoidReward(voidTarget.id, voidReason)
    if (success) {
      setVoidTarget(null)
      setVoidReason('')
    }
  }

  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent className='flex max-h-[calc(100dvh-2rem)] flex-col max-sm:h-dvh max-sm:w-screen max-sm:max-w-none max-sm:rounded-none max-sm:p-4 sm:max-w-5xl'>
          <DialogHeader>
            <DialogTitle>
              {isAdmin ? t('Admin reward ledger') : t('Reward History')}
            </DialogTitle>
            <DialogDescription>
              {t('View pending, available, transferred, and voided rewards.')}
            </DialogDescription>
          </DialogHeader>

          <div className='min-h-0 flex-1 space-y-3 sm:space-y-4'>
            {isAdmin && summary ? (
              <div className='grid grid-cols-2 gap-2 rounded-lg border p-3 text-center sm:grid-cols-3 lg:grid-cols-6'>
                {[
                  [t('Total Earned'), formatQuota(summary.total_reward_quota)],
                  [
                    t('Available Rewards'),
                    formatQuota(summary.available_reward_quota),
                  ],
                  [
                    t('Pending Settlement'),
                    formatQuota(summary.pending_reward_quota),
                  ],
                  [
                    t('Transferred'),
                    formatQuota(summary.transferred_reward_quota),
                  ],
                  [t('Voided'), formatQuota(summary.voided_reward_quota)],
                  [
                    t('Effective Referrals'),
                    String(summary.effective_invitee_count),
                  ],
                ].map(([label, value]) => (
                  <div key={label}>
                    <div className='text-muted-foreground truncate text-[10px] font-medium tracking-wider uppercase'>
                      {label}
                    </div>
                    <div className='mt-0.5 truncate text-sm font-semibold tabular-nums'>
                      {value}
                    </div>
                  </div>
                ))}
              </div>
            ) : null}

            <div className='grid gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'>
              <div className='relative min-w-0'>
                <Search className='text-muted-foreground absolute top-1/2 left-3 size-4 -translate-y-1/2' />
                <Input
                  value={keyword}
                  onChange={(event) => handleSearch(event.target.value)}
                  placeholder={t('Search by order, user, or reason...')}
                  className='h-9 pl-10'
                />
              </div>
              <Select
                value={status}
                onValueChange={(value) =>
                  value !== null &&
                  handleStatusChange(value as AffiliateRewardStatus | 'all')
                }
              >
                <SelectTrigger className='h-9'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All statuses')}</SelectItem>
                    <SelectItem value='pending'>{t('Pending')}</SelectItem>
                    <SelectItem value='available'>{t('Available')}</SelectItem>
                    <SelectItem value='transferred'>
                      {t('Transferred')}
                    </SelectItem>
                    <SelectItem value='voided'>{t('Voided')}</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Select
                value={sourceType}
                onValueChange={(value) =>
                  value !== null &&
                  handleSourceTypeChange(
                    value as AffiliateRewardSourceType | 'all'
                  )
                }
              >
                <SelectTrigger className='h-9'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All sources')}</SelectItem>
                    <SelectItem value='topup'>{t('Top-up order')}</SelectItem>
                    <SelectItem value='subscription_order'>
                      {t('Subscription order')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Select
                value={triggerType}
                onValueChange={(value) =>
                  value !== null &&
                  handleTriggerTypeChange(
                    value as AffiliateRewardTriggerType | 'all'
                  )
                }
              >
                <SelectTrigger className='h-9'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All triggers')}</SelectItem>
                    <SelectItem value='first_topup'>
                      {t('First top-up')}
                    </SelectItem>
                    <SelectItem value='recurring_topup'>
                      {t('Recurring top-up')}
                    </SelectItem>
                    <SelectItem value='subscription_order'>
                      {t('Subscription order')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <Select
                value={paymentProvider}
                onValueChange={(value) =>
                  value !== null && handlePaymentProviderChange(value)
                }
              >
                <SelectTrigger className='h-9'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    <SelectItem value='all'>{t('All providers')}</SelectItem>
                    <SelectItem value='epay'>{t('Epay')}</SelectItem>
                    <SelectItem value='stripe'>{t('Stripe')}</SelectItem>
                    <SelectItem value='creem'>{t('Creem')}</SelectItem>
                    <SelectItem value='waffo'>{t('Waffo')}</SelectItem>
                    <SelectItem value='waffo_pancake'>
                      {t('Waffo Pancake')}
                    </SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <CompactDateTimeRangePicker
                start={startTime}
                end={endTime}
                onChange={handleTimeRangeChange}
                className='h-9'
              />
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
                <SelectTrigger className='h-9'>
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

            <ScrollArea className='h-[calc(100dvh-18rem)] pr-3 sm:h-[500px] sm:pr-4'>
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
                    {t(
                      'Reward records will appear here after eligible top-ups.'
                    )}
                  </p>
                </div>
              ) : (
                <div className='space-y-3'>
                  {records.map((record) => (
                    <RewardRecordItem
                      key={record.id}
                      record={record}
                      isAdmin={isAdmin}
                      onVoid={(nextRecord) => {
                        setVoidTarget(nextRecord)
                        setVoidReason('')
                      }}
                      voiding={voiding}
                    />
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

      <AlertDialog
        open={!!voidTarget}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            setVoidTarget(null)
            setVoidReason('')
          }
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Void reward')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Voiding a reward will reverse its ledger balance.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <Input
            value={voidReason}
            onChange={(event) => setVoidReason(event.target.value)}
            placeholder={t('Void reason')}
            disabled={voiding}
          />
          <AlertDialogFooter>
            <AlertDialogCancel disabled={voiding}>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleConfirmVoid}
              disabled={voiding || voidReason.trim() === ''}
            >
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
