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
import { getAffiliateRewards, isApiSuccess } from '../api'
import type { AffiliateRewardRecord } from '../types'

type UseAffiliateRewardsOptions = {
  initialPage?: number
  initialPageSize?: number
  enabled?: boolean
}

export function useAffiliateRewards(options: UseAffiliateRewardsOptions = {}) {
  const { initialPage = 1, initialPageSize = 10, enabled = true } = options
  const [records, setRecords] = useState<AffiliateRewardRecord[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(initialPage)
  const [pageSize, setPageSize] = useState(initialPageSize)
  const [loading, setLoading] = useState(false)

  const fetchRewards = useCallback(async () => {
    if (!enabled) return

    setLoading(true)
    try {
      const response = await getAffiliateRewards(page, pageSize)
      if (isApiSuccess(response) && response.data) {
        setRecords(response.data.items || [])
        setTotal(response.data.total || 0)
      } else {
        toast.error(
          response.message || i18next.t('Failed to load reward history')
        )
        setRecords([])
        setTotal(0)
      }
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to fetch affiliate rewards:', error)
      toast.error(i18next.t('Failed to load reward history'))
      setRecords([])
      setTotal(0)
    } finally {
      setLoading(false)
    }
  }, [enabled, page, pageSize])

  const handlePageChange = useCallback((newPage: number) => {
    setPage(newPage)
  }, [])

  const handlePageSizeChange = useCallback((newPageSize: number) => {
    setPageSize(newPageSize)
    setPage(1)
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchRewards()
  }, [fetchRewards])

  return {
    records,
    total,
    page,
    pageSize,
    loading,
    handlePageChange,
    handlePageSizeChange,
    refresh: fetchRewards,
  }
}
