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

import React, { useEffect, useState, useRef } from 'react';
import {
  Banner,
  Button,
  Col,
  Form,
  Row,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import {
  compareObjects,
  API,
  showError,
  showSuccess,
  showWarning,
} from '../../../helpers';

const isTruthyOption = (value) => value === true || value === 'true';

export default function SettingsCreditLimit(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    QuotaForNewUser: '',
    PreConsumedQuota: '',
    QuotaForInviter: '',
    QuotaForInvitee: '',
    'quota_setting.enable_free_model_pre_consume': true,
    'affiliate_setting.enabled': false,
    'affiliate_setting.attribution_window_days': 30,
    'affiliate_setting.min_topup_quota': 0,
    'affiliate_setting.settlement_delay_days': 0,
    'affiliate_setting.first_reward_enabled': true,
    'affiliate_setting.first_reward_rate': 0.1,
    'affiliate_setting.first_reward_cap_quota': 0,
    'affiliate_setting.recurring_reward_enabled': false,
    'affiliate_setting.recurring_reward_rate': 0.02,
    'affiliate_setting.recurring_window_days': 90,
    'affiliate_setting.recurring_max_count': 0,
    'affiliate_setting.recurring_cap_per_invitee': 0,
    'affiliate_setting.include_subscription_orders': true,
  });
  const refForm = useRef();
  const [inputsRow, setInputsRow] = useState(inputs);
  const complianceConfirmed =
    isTruthyOption(props.options?.['payment_setting.compliance_confirmed']) &&
    props.options?.['payment_setting.compliance_terms_version'] === 'v1';

  const updateInput = (key, value) => {
    setInputs((prev) => ({
      ...prev,
      [key]: value,
    }));
  };
  const affiliateEnabled = isTruthyOption(inputs['affiliate_setting.enabled']);
  const firstRewardEnabled = isTruthyOption(
    inputs['affiliate_setting.first_reward_enabled'],
  );
  const recurringRewardEnabled = isTruthyOption(
    inputs['affiliate_setting.recurring_reward_enabled'],
  );
  const disableNewAffiliateRewards = !complianceConfirmed && !affiliateEnabled;
  const disableFirstRewardSwitch =
    !affiliateEnabled || (!complianceConfirmed && !firstRewardEnabled);
  const disableRecurringRewardSwitch =
    !affiliateEnabled || (!complianceConfirmed && !recurringRewardEnabled);
  const disableFirstRewardFields =
    !affiliateEnabled || !firstRewardEnabled || !complianceConfirmed;
  const disableRecurringRewardFields =
    !affiliateEnabled || !recurringRewardEnabled || !complianceConfirmed;

  function onSubmit() {
    const updateArray = compareObjects(inputs, inputsRow);
    if (!updateArray.length) return showWarning(t('你似乎并没有修改什么'));
    const requestQueue = updateArray.map((item) => {
      let value = '';
      if (typeof inputs[item.key] === 'boolean') {
        value = String(inputs[item.key]);
      } else {
        value = inputs[item.key];
      }
      return API.put('/api/option/', {
        key: item.key,
        value,
      });
    });
    setLoading(true);
    Promise.all(requestQueue)
      .then((res) => {
        if (requestQueue.length === 1) {
          if (res.includes(undefined)) return;
        } else if (requestQueue.length > 1) {
          if (res.includes(undefined))
            return showError(t('部分保存失败，请重试'));
        }
        showSuccess(t('保存成功'));
        props.refresh();
      })
      .catch(() => {
        showError(t('保存失败，请重试'));
      })
      .finally(() => {
        setLoading(false);
      });
  }

  useEffect(() => {
    const currentInputs = {};
    for (let key in props.options) {
      if (Object.keys(inputs).includes(key)) {
        currentInputs[key] = props.options[key];
      }
    }
    setInputs(currentInputs);
    setInputsRow(structuredClone(currentInputs));
    refForm.current.setValues(currentInputs);
  }, [props.options]);
  return (
    <>
      <Spin spinning={loading}>
        {!complianceConfirmed && (
          <Banner
            type='warning'
            description={t(
              '设置非零邀请奖励额度前，需要先在支付设置中确认合规声明。',
            )}
            closeIcon={null}
            className='!rounded-lg mb-3'
          />
        )}
        <Form
          values={inputs}
          getFormApi={(formAPI) => (refForm.current = formAPI)}
          style={{ marginBottom: 15 }}
        >
          <Form.Section text={t('额度设置')}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('新用户初始额度')}
                  field={'QuotaForNewUser'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  placeholder={''}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForNewUser: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('请求预扣费额度')}
                  field={'PreConsumedQuota'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={t('请求结束后多退少补')}
                  placeholder={''}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      PreConsumedQuota: String(value),
                    })
                  }
                />
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.InputNumber
                  label={t('邀请新用户奖励额度')}
                  field={'QuotaForInviter'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={
                    !complianceConfirmed ? t('非零值需先确认合规声明') : ''
                  }
                  placeholder={t('例如：2000')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForInviter: String(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Col xs={24} sm={12} md={8} lg={8} xl={6}>
                <Form.InputNumber
                  label={t('新用户使用邀请码奖励额度')}
                  field={'QuotaForInvitee'}
                  step={1}
                  min={0}
                  suffix={'Token'}
                  extraText={
                    !complianceConfirmed ? t('非零值需先确认合规声明') : ''
                  }
                  placeholder={t('例如：1000')}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      QuotaForInvitee: String(value),
                    })
                  }
                />
              </Col>
            </Row>
            <Row>
              <Col>
                <Form.Switch
                  label={t('对免费模型启用预消耗')}
                  field={'quota_setting.enable_free_model_pre_consume'}
                  extraText={t(
                    '开启后，对免费模型（倍率为0，或者价格为0）的模型也会预消耗额度',
                  )}
                  onChange={(value) =>
                    setInputs({
                      ...inputs,
                      'quota_setting.enable_free_model_pre_consume': value,
                    })
                  }
                />
              </Col>
            </Row>

            <div className='mt-6'>
              <Typography.Text strong>{t('邀请返利设置')}</Typography.Text>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Switch
                    label={t('启用邀请返利')}
                    field={'affiliate_setting.enabled'}
                    size='default'
                    checkedText='｜'
                    uncheckedText='〇'
                    disabled={disableNewAffiliateRewards}
                    extraText={
                      !complianceConfirmed ? t('需先确认合规声明') : ''
                    }
                    onChange={(value) =>
                      updateInput('affiliate_setting.enabled', value)
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('首充返利窗口天数')}
                    field={'affiliate_setting.attribution_window_days'}
                    min={0}
                    step={1}
                    extraText={t(
                      '首充返利仅在注册后的这些天数内生效。填 0 表示不限制。',
                    )}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.attribution_window_days',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('最低充值额度')}
                    field={'affiliate_setting.min_topup_quota'}
                    min={0}
                    step={1}
                    extraText={t('低于该额度不会产生返利')}
                    onChange={(value) =>
                      updateInput('affiliate_setting.min_topup_quota', value)
                    }
                  />
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('结算延迟天数')}
                    field={'affiliate_setting.settlement_delay_days'}
                    min={0}
                    step={1}
                    extraText={t('奖励先进入待结算，到期后可划转')}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.settlement_delay_days',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Switch
                    label={t('启用首充返利')}
                    field={'affiliate_setting.first_reward_enabled'}
                    size='default'
                    checkedText='｜'
                    uncheckedText='〇'
                    disabled={disableFirstRewardSwitch}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.first_reward_enabled',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('首充返利比例')}
                    field={'affiliate_setting.first_reward_rate'}
                    min={0}
                    step={0.01}
                    precision={4}
                    disabled={disableFirstRewardFields}
                    extraText={t('填写小数，例如 0.1 表示 10%')}
                    onChange={(value) =>
                      updateInput('affiliate_setting.first_reward_rate', value)
                    }
                  />
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('首充返利封顶额度')}
                    field={'affiliate_setting.first_reward_cap_quota'}
                    min={0}
                    step={1}
                    disabled={disableFirstRewardFields}
                    extraText={t('填 0 表示不封顶')}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.first_reward_cap_quota',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Switch
                    label={t('启用长期返利')}
                    field={'affiliate_setting.recurring_reward_enabled'}
                    size='default'
                    checkedText='｜'
                    uncheckedText='〇'
                    disabled={disableRecurringRewardSwitch}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.recurring_reward_enabled',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('长期返利比例')}
                    field={'affiliate_setting.recurring_reward_rate'}
                    min={0}
                    step={0.01}
                    precision={4}
                    disabled={disableRecurringRewardFields}
                    extraText={t('填写小数，例如 0.02 表示 2%')}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.recurring_reward_rate',
                        value,
                      )
                    }
                  />
                </Col>
              </Row>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('长期返利窗口天数')}
                    field={'affiliate_setting.recurring_window_days'}
                    min={0}
                    step={1}
                    disabled={disableRecurringRewardFields}
                    extraText={t(
                      '长期返利仅在注册后的这些天数内生效。填 0 表示不限制。',
                    )}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.recurring_window_days',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('长期返利订单数限制')}
                    field={'affiliate_setting.recurring_max_count'}
                    min={0}
                    step={1}
                    disabled={disableRecurringRewardFields}
                    extraText={t('填 0 表示不限次数')}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.recurring_max_count',
                        value,
                      )
                    }
                  />
                </Col>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.InputNumber
                    label={t('单个邀请长期返利封顶')}
                    field={'affiliate_setting.recurring_cap_per_invitee'}
                    min={0}
                    step={1}
                    disabled={disableRecurringRewardFields}
                    extraText={t('填 0 表示不封顶')}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.recurring_cap_per_invitee',
                        value,
                      )
                    }
                  />
                </Col>
              </Row>
              <Row>
                <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                  <Form.Switch
                    label={t('订阅订单参与返利')}
                    field={'affiliate_setting.include_subscription_orders'}
                    size='default'
                    checkedText='｜'
                    uncheckedText='〇'
                    disabled={!affiliateEnabled}
                    onChange={(value) =>
                      updateInput(
                        'affiliate_setting.include_subscription_orders',
                        value,
                      )
                    }
                  />
                </Col>
              </Row>
            </div>

            <Row>
              <Button size='default' onClick={onSubmit}>
                {t('保存额度设置')}
              </Button>
            </Row>
          </Form.Section>
        </Form>
      </Spin>
    </>
  );
}
