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
import { useCallback, useEffect, useState } from 'react'
import i18next from 'i18next'
import { toast } from 'sonner'
import {
  getAdminAffiliateRewards,
  getAffiliateRewards,
  isApiSuccess,
  voidAffiliateReward,
} from '../api'
import type {
  AffiliateRewardFilters,
  AffiliateRewardRecord,
  AffiliateRewardStatus,
  AffiliateRewardSummary,
  AffiliateRewardTriggerType,
  AffiliateRewardSourceType,
} from '../types'

type UseAffiliateRewardsOptions = {
  initialPage?: number
  initialPageSize?: number
  enabled?: boolean
  admin?: boolean
}

export function useAffiliateRewards(options: UseAffiliateRewardsOptions = {}) {
  const {
    initialPage = 1,
    initialPageSize = 10,
    enabled = true,
    admin = false,
  } = options
  const [records, setRecords] = useState<AffiliateRewardRecord[]>([])
  const [total, setTotal] = useState(0)
  const [summary, setSummary] = useState<AffiliateRewardSummary | null>(null)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [keyword, setKeyword] = useState('')
  const [status, setStatus] = useState<AffiliateRewardStatus | 'all'>('all')
  const [triggerType, setTriggerType] = useState<
    AffiliateRewardTriggerType | 'all'
  >('all')
  const [sourceType, setSourceType] = useState<
    AffiliateRewardSourceType | 'all'
  >('all')
  const [paymentProvider, setPaymentProvider] = useState('all')
  const [startTime, setStartTime] = useState<Date | undefined>()
  const [endTime, setEndTime] = useState<Date | undefined>()
  const [loading, setLoading] = useState(false)
  const [voiding, setVoiding] = useState(false)

  const fetchRewards = useCallback(async () => {
    if (!enabled) return

    const filters: AffiliateRewardFilters = {
      keyword,
      status,
      trigger_type: triggerType,
      source_type: sourceType,
      payment_provider: paymentProvider,
      start_time: startTime
        ? Math.floor(startTime.getTime() / 1000)
        : undefined,
      end_time: endTime ? Math.floor(endTime.getTime() / 1000) : undefined,
    }
    setLoading(true)
    try {
      const response = admin
        ? await getAdminAffiliateRewards(page, pageSize, filters)
        : await getAffiliateRewards(page, pageSize, filters)
      if (isApiSuccess(response) && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
        setSummary(response.data.summary || null)
      } else {
        toast.error(
          response.message || i18next.t('Failed to load reward history')
        )
        setRecords([])
        setTotal(0)
        setSummary(null)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate rewards:', error)
      toast.error(i18next.t('Failed to load reward history'))
      setRecords([])
      setTotal(0)
      setSummary(null)
    } finally {
      setLoading(false)
    }
  }, [
    admin,
    enabled,
    endTime,
    keyword,
    page,
    pageSize,
    paymentProvider,
    sourceType,
    startTime,
    status,
    triggerType,
  ])

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1)
  }, [])

  const handleSearch = useCallback((nextKeyword: string) => {
    setKeyword(nextKeyword)
    setPage(1)
  }, [])

  const handleStatusChange = useCallback(
    (nextStatus: AffiliateRewardStatus | 'all') => {
      setStatus(nextStatus)
      setPage(1)
    },
    []
  )

  const handleTriggerTypeChange = useCallback(
    (nextTriggerType: AffiliateRewardTriggerType | 'all') => {
      setTriggerType(nextTriggerType)
      setPage(1)
    },
    []
  )

  const handleSourceTypeChange = useCallback(
    (nextSourceType: AffiliateRewardSourceType | 'all') => {
      setSourceType(nextSourceType)
      setPage(1)
    },
    []
  )

  const handlePaymentProviderChange = useCallback(
    (nextPaymentProvider: string) => {
      setPaymentProvider(nextPaymentProvider)
      setPage(1)
    },
    []
  )

  const handleTimeRangeChange = useCallback(
    (range: { start?: Date; end?: Date }) => {
      setStartTime(range.start)
      setEndTime(range.end)
      setPage(1)
    },
    []
  )

  const handleVoidReward = useCallback(
    async (id: number, reason: string) => {
      if (!admin) {
        toast.error(i18next.t('Admin access required'))
        return false
      }
      setVoiding(true)
      try {
        const response = await voidAffiliateReward(id, reason)
        if (isApiSuccess(response)) {
          toast.success(i18next.t('Reward voided successfully'))
          await fetchRewards()
          return true
        }
        toast.error(response.message || i18next.t('Failed to void reward'))
        return false
      } catch (error) {
        // eslint-disable-next-line no-console
        console.error('Failed to void affiliate reward:', error)
        toast.error(i18next.t('Failed to void reward'))
        return false
      } finally {
        setVoiding(false)
      }
    },
    [admin, fetchRewards]
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchRewards()
  }, [fetchRewards])

  return {
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
    refresh: fetchRewards,
  }
}
