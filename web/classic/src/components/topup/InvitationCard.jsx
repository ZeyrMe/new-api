/*
Copyright (C) 2025 QuantumNous

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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Avatar,
  Typography,
  Card,
  Button,
  Input,
  Badge,
  Space,
  Modal,
  Table,
  Select,
  Empty,
} from '@douyinfe/semi-ui';
import { IconSearch } from '@douyinfe/semi-icons';
import {
  Copy,
  Users,
  BarChart2,
  TrendingUp,
  Gift,
  Zap,
  Clock3,
  ListChecks,
  Ban,
} from 'lucide-react';
import {
  IllustrationNoResult,
  IllustrationNoResultDark,
} from '@douyinfe/semi-illustrations';
import {
  API,
  isAdmin,
  showError,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { useIsMobile } from '../../hooks/common/useIsMobile';

const { Text } = Typography;

const rewardStatusConfig = {
  pending: { type: 'warning', label: '待结算' },
  available: { type: 'success', label: '可划转' },
  transferred: { type: 'primary', label: '已划转' },
  voided: { type: 'danger', label: '已作废' },
};

const triggerLabels = {
  first_topup: '首次充值',
  recurring_topup: '长期充值',
  subscription_order: '订阅订单',
};

const sourceLabels = {
  topup: '充值订单',
  subscription_order: '订阅订单',
};

const paymentProviderLabels = {
  epay: 'Epay',
  stripe: 'Stripe',
  creem: 'Creem',
  waffo: 'Waffo',
  waffo_pancake: 'Waffo Pancake',
};

const formatRate = (rate) => {
  const value = Number(rate || 0) * 100;
  return `${value.toFixed(2).replace(/\.?0+$/, '')}%`;
};

const formatTimeOrDash = (timestamp) =>
  timestamp > 0 ? timestamp2string(timestamp) : '-';

const buildRewardQuery = (page, pageSize, filters) => {
  const params = new URLSearchParams({
    p: String(page),
    page_size: String(pageSize),
  });
  Object.entries(filters).forEach(([key, value]) => {
    if (
      value === undefined ||
      value === null ||
      value === '' ||
      value === 'all'
    ) {
      return;
    }
    params.append(key, String(value));
  });
  return params.toString();
};

function AffiliateRewardLedgerModal({
  visible,
  onCancel,
  t,
  renderQuota,
  onChanged,
}) {
  const userIsAdmin = useMemo(() => isAdmin(), []);
  const isMobile = useIsMobile();
  const [loading, setLoading] = useState(false);
  const [voiding, setVoiding] = useState(false);
  const [records, setRecords] = useState([]);
  const [summary, setSummary] = useState(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [keyword, setKeyword] = useState('');
  const [status, setStatus] = useState('all');
  const [triggerType, setTriggerType] = useState('all');
  const [sourceType, setSourceType] = useState('all');
  const [paymentProvider, setPaymentProvider] = useState('all');

  const loadRewards = useCallback(
    async (currentPage = page, currentPageSize = pageSize) => {
      setLoading(true);
      try {
        const filters = {
          keyword,
          status,
          trigger_type: triggerType,
          source_type: sourceType,
          payment_provider: paymentProvider,
        };
        const base = userIsAdmin
          ? '/api/user/aff_rewards/admin'
          : '/api/user/aff_rewards';
        const res = await API.get(
          `${base}?${buildRewardQuery(currentPage, currentPageSize, filters)}`,
        );
        const { success, message, data } = res.data;
        if (success) {
          setRecords(data.items || []);
          setTotal(data.total || 0);
          setSummary(data.summary || null);
        } else {
          showError(message || t('加载失败'));
        }
      } catch (error) {
        showError(t('加载返利账本失败'));
      } finally {
        setLoading(false);
      }
    },
    [
      keyword,
      page,
      pageSize,
      paymentProvider,
      sourceType,
      status,
      t,
      triggerType,
      userIsAdmin,
    ],
  );

  useEffect(() => {
    if (visible) {
      loadRewards(page, pageSize);
    }
  }, [visible, loadRewards, page, pageSize]);

  const handleCancel = () => {
    onCancel();
    onChanged?.();
  };

  const resetPageAndSet = (setter) => (value) => {
    setter(value);
    setPage(1);
  };

  const handleVoidReward = useCallback(
    (record) => {
      let reason = '';
      Modal.confirm({
        title: t('作废奖励'),
        content: (
          <div className='space-y-3'>
            <Text>{t('作废后会按账务规则反冲该返利余额。')}</Text>
            <Input
              placeholder={t('请输入作废原因')}
              onChange={(value) => {
                reason = value;
              }}
            />
          </div>
        ),
        okText: t('确认'),
        cancelText: t('取消'),
        onOk: async () => {
          const trimmedReason = reason.trim();
          if (!trimmedReason) {
            showError(t('作废原因不能为空'));
            return Promise.reject();
          }
          setVoiding(true);
          try {
            const res = await API.post(
              `/api/user/aff_rewards/${record.id}/void`,
              {
                reason: trimmedReason,
              },
            );
            if (res.data?.success) {
              showSuccess(t('作废成功'));
              await loadRewards(page, pageSize);
              await onChanged?.();
            } else {
              showError(res.data?.message || t('作废失败'));
              return Promise.reject();
            }
          } catch (error) {
            showError(t('作废失败'));
            return Promise.reject();
          } finally {
            setVoiding(false);
          }
        },
      });
    },
    [loadRewards, onChanged, page, pageSize, t],
  );

  const columns = useMemo(() => {
    const result = [
      ...(userIsAdmin
        ? [
            {
              title: t('邀请人'),
              dataIndex: 'inviter_id',
              key: 'inviter_id',
              render: (value) => <Text copyable>{value}</Text>,
            },
            {
              title: t('被邀请人'),
              dataIndex: 'invitee_id',
              key: 'invitee_id',
              render: (value) => <Text copyable>{value}</Text>,
            },
          ]
        : []),
      {
        title: t('订单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (value) => <Text copyable>{value}</Text>,
      },
      {
        title: t('触发类型'),
        dataIndex: 'trigger_type',
        key: 'trigger_type',
        render: (value) => t(triggerLabels[value] || value || '-'),
      },
      {
        title: t('来源'),
        dataIndex: 'source_type',
        key: 'source_type',
        render: (value) => t(sourceLabels[value] || value || '-'),
      },
      {
        title: t('支付渠道'),
        dataIndex: 'payment_provider',
        key: 'payment_provider',
        render: (value) => t(paymentProviderLabels[value] || value || '-'),
      },
      {
        title: t('奖励比例'),
        dataIndex: 'reward_rate',
        key: 'reward_rate',
        render: (value) => <Text>{formatRate(value)}</Text>,
      },
      {
        title: t('奖励额度'),
        dataIndex: 'reward_quota',
        key: 'reward_quota',
        render: (value) => (
          <Text type='success'>{renderQuota(value || 0)}</Text>
        ),
      },
      {
        title: t('剩余额度'),
        key: 'remaining_quota',
        render: (_, record) =>
          renderQuota(
            Math.max(
              0,
              (record.reward_quota || 0) - (record.transferred_quota || 0),
            ),
          ),
      },
      {
        title: t('状态'),
        dataIndex: 'status',
        key: 'status',
        render: (value) => {
          const config = rewardStatusConfig[value] || {
            type: 'primary',
            label: value,
          };
          return (
            <span className='flex items-center gap-2'>
              <Badge dot type={config.type} />
              <span>{t(config.label)}</span>
            </span>
          );
        },
      },
      {
        title: t('可用时间'),
        dataIndex: 'eligible_at',
        key: 'eligible_at',
        render: formatTimeOrDash,
      },
      {
        title: t('创建时间'),
        dataIndex: 'created_at',
        key: 'created_at',
        render: formatTimeOrDash,
      },
    ];

    if (userIsAdmin) {
      result.push({
        title: t('操作'),
        key: 'action',
        fixed: 'right',
        render: (_, record) =>
          record.status === 'voided' ? (
            record.void_reason ? (
              <Text type='tertiary'>{record.void_reason}</Text>
            ) : null
          ) : (
            <Button
              size='small'
              theme='outline'
              type='danger'
              icon={<Ban size={12} />}
              loading={voiding}
              onClick={() => handleVoidReward(record)}
            >
              {t('作废')}
            </Button>
          ),
      });
    }

    return result;
  }, [t, userIsAdmin, renderQuota, voiding, handleVoidReward]);

  return (
    <Modal
      title={userIsAdmin ? t('返利账本') : t('收益明细')}
      visible={visible}
      onCancel={handleCancel}
      footer={null}
      size={isMobile ? 'full-width' : 'large'}
    >
      {userIsAdmin && summary && (
        <div className='grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2 mb-3'>
          {[
            [t('历史收益'), renderQuota(summary.total_reward_quota || 0)],
            [t('可用收益'), renderQuota(summary.available_reward_quota || 0)],
            [t('待结算'), renderQuota(summary.pending_reward_quota || 0)],
            [t('已划转'), renderQuota(summary.transferred_reward_quota || 0)],
            [t('已作废'), renderQuota(summary.voided_reward_quota || 0)],
            [t('有效邀请'), summary.effective_invitee_count || 0],
          ].map(([label, value]) => (
            <Card
              key={label}
              className='!rounded-xl text-center'
              bodyStyle={{ padding: 12 }}
            >
              <div className='text-xs text-gray-500'>{label}</div>
              <div className='font-semibold mt-1'>{value}</div>
            </Card>
          ))}
        </div>
      )}
      <div className='grid grid-cols-1 md:grid-cols-3 lg:grid-cols-5 gap-2 mb-3'>
        <Input
          prefix={<IconSearch />}
          placeholder={t('搜索订单、用户或原因')}
          value={keyword}
          onChange={resetPageAndSet(setKeyword)}
          showClear
        />
        <Select
          value={status}
          onChange={resetPageAndSet(setStatus)}
          optionList={[
            { value: 'all', label: t('全部状态') },
            { value: 'pending', label: t('待结算') },
            { value: 'available', label: t('可划转') },
            { value: 'transferred', label: t('已划转') },
            { value: 'voided', label: t('已作废') },
          ]}
        />
        <Select
          value={triggerType}
          onChange={resetPageAndSet(setTriggerType)}
          optionList={[
            { value: 'all', label: t('全部触发') },
            { value: 'first_topup', label: t('首次充值') },
            { value: 'recurring_topup', label: t('长期充值') },
            { value: 'subscription_order', label: t('订阅订单') },
          ]}
        />
        <Select
          value={sourceType}
          onChange={resetPageAndSet(setSourceType)}
          optionList={[
            { value: 'all', label: t('全部来源') },
            { value: 'topup', label: t('充值订单') },
            { value: 'subscription_order', label: t('订阅订单') },
          ]}
        />
        <Select
          value={paymentProvider}
          onChange={resetPageAndSet(setPaymentProvider)}
          optionList={[
            { value: 'all', label: t('全部渠道') },
            { value: 'epay', label: 'Epay' },
            { value: 'stripe', label: 'Stripe' },
            { value: 'creem', label: 'Creem' },
            { value: 'waffo', label: 'Waffo' },
            { value: 'waffo_pancake', label: 'Waffo Pancake' },
          ]}
        />
      </div>
      <Table
        columns={columns}
        dataSource={records}
        loading={loading}
        rowKey='id'
        size='small'
        scroll={{ x: 1200 }}
        pagination={{
          currentPage: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOpts: [10, 20, 50, 100],
          onPageChange: setPage,
          onPageSizeChange: (nextPageSize) => {
            setPageSize(nextPageSize);
            setPage(1);
          },
        }}
        empty={
          <Empty
            image={<IllustrationNoResult style={{ width: 150, height: 150 }} />}
            darkModeImage={
              <IllustrationNoResultDark style={{ width: 150, height: 150 }} />
            }
            description={t('暂无返利记录')}
            style={{ padding: 30 }}
          />
        }
      />
    </Modal>
  );
}

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  setOpenTransfer,
  affLink,
  handleAffLinkClick,
  onRewardChanged,
  complianceConfirmed = true,
}) => {
  const [ledgerVisible, setLedgerVisible] = useState(false);
  const availableQuota =
    userState?.user?.aff_available_quota ?? userState?.user?.aff_quota ?? 0;
  const pendingQuota = userState?.user?.aff_pending_quota || 0;
  const historyQuota = userState?.user?.aff_history_quota || 0;
  const effectiveCount =
    userState?.user?.aff_effective_count ?? userState?.user?.aff_count ?? 0;

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      {/* 卡片头部 */}
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='green' className='mr-3 shadow-md'>
          <Gift size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('邀请奖励')}
          </Typography.Text>
          <div className='text-xs'>{t('邀请好友获得额外奖励')}</div>
        </div>
      </div>

      {/* 收益展示区域 */}
      <Space vertical style={{ width: '100%' }}>
        {/* 统计数据统一卡片 */}
        <Card
          className='!rounded-xl w-full'
          cover={
            <div
              className='relative h-30'
              style={{
                '--palette-primary-darkerChannel': '0 75 80',
                backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
                backgroundSize: 'cover',
                backgroundPosition: 'center',
                backgroundRepeat: 'no-repeat',
              }}
            >
              {/* 标题和按钮 */}
              <div className='relative z-10 h-full flex flex-col justify-between p-4'>
                <div className='flex justify-between items-center gap-3 flex-wrap'>
                  <Text strong style={{ color: 'white', fontSize: '16px' }}>
                    {t('收益统计')}
                  </Text>
                  <div className='flex items-center gap-2 flex-wrap justify-end'>
                    <Button
                      type='tertiary'
                      theme='solid'
                      size='small'
                      onClick={() => setLedgerVisible(true)}
                      className='!rounded-lg'
                    >
                      <ListChecks size={12} className='mr-1' />
                      {t('收益明细')}
                    </Button>
                    <Button
                      type='primary'
                      theme='solid'
                      size='small'
                      disabled={
                        !complianceConfirmed ||
                        !availableQuota ||
                        availableQuota <= 0
                      }
                      onClick={() => setOpenTransfer(true)}
                      className='!rounded-lg'
                    >
                      <Zap size={12} className='mr-1' />
                      {t('划转到余额')}
                    </Button>
                  </div>
                </div>
                {!complianceConfirmed && (
                  <Text
                    style={{
                      color: 'rgba(255,255,255,0.8)',
                      fontSize: 12,
                    }}
                  >
                    {t('邀请奖励划转已禁用，管理员需先确认合规声明。')}
                  </Text>
                )}

                {/* 统计数据 */}
                <div className='grid grid-cols-2 sm:grid-cols-4 gap-4 mt-4'>
                  {/* 可用收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(availableQuota)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <TrendingUp
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('可用收益')}
                      </Text>
                    </div>
                  </div>

                  {/* 待结算收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(pendingQuota)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <Clock3
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('待结算')}
                      </Text>
                    </div>
                  </div>

                  {/* 总收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(historyQuota)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <BarChart2
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('历史收益')}
                      </Text>
                    </div>
                  </div>

                  {/* 邀请人数 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {effectiveCount}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <Users
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('有效邀请')}
                      </Text>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          }
        >
          {/* 邀请链接部分 */}
          <Input
            value={affLink}
            readonly
            className='!rounded-lg'
            prefix={t('邀请链接')}
            suffix={
              <Button
                type='primary'
                theme='solid'
                onClick={handleAffLinkClick}
                icon={<Copy size={14} />}
                className='!rounded-lg'
              >
                {t('复制')}
              </Button>
            }
          />
        </Card>

        {/* 奖励说明 */}
        <Card
          className='!rounded-xl w-full'
          title={<Text type='tertiary'>{t('奖励说明')}</Text>}
        >
          <div className='space-y-3'>
            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('邀请好友注册，好友充值后您可获得相应奖励')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('通过划转功能将奖励额度转入到您的账户余额中')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('邀请的好友越多，获得的奖励越多')}
              </Text>
            </div>
          </div>
        </Card>
      </Space>
      <AffiliateRewardLedgerModal
        visible={ledgerVisible}
        onCancel={() => setLedgerVisible(false)}
        t={t}
        renderQuota={renderQuota}
        onChanged={onRewardChanged}
      />
    </Card>
  );
};

export default InvitationCard;
