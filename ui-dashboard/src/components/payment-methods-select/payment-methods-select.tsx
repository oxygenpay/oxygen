import "./payment-methods-select.scss";

import * as React from "react";
import {useAsyncFn, useMount} from "react-use";
import bevis from "src/utils/bevis";
import {Checkbox, Row, Typography} from "antd";
import {BlockchainTicker} from "src/types/index";
import merchantProvider from "src/providers/merchant-provider";
import Icon from "src/components/icon/icon";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import useSharedMerchant from "src/hooks/use-merchant";

const b = bevis("payment-methods-select");

const PaymentMethodsSelect: React.FC = () => {
    const {merchant, getMerchant} = useSharedMerchant();
    const {merchantId} = useSharedMerchantId();
    const [supportedMethodsReqState, updateSupportedMethods] = useAsyncFn(merchantProvider.updateSupportedMethods);

    const onChange = (ticker: BlockchainTicker) => {
        if (!merchantId || !merchant?.supportedPaymentMethods) {
            return;
        }

        const newPaymentMethods = [...merchant.supportedPaymentMethods];
        const index = newPaymentMethods.findIndex((item) => item.ticker === ticker);
        newPaymentMethods[index].enabled = !newPaymentMethods[index].enabled;

        const supportedPaymentMethods = newPaymentMethods.filter((item) => item.enabled).map((item) => item.ticker);

        updateSupportedMethods(merchantId, {supportedPaymentMethods});
    };

    const updateMerchant = async () => {
        if (!merchantId) {
            return;
        }

        await getMerchant(merchantId);
    };

    useMount(async () => {
        await updateMerchant();
    });

    React.useEffect(() => {
        updateMerchant();
    }, [merchantId]);

    return (
        <>
            <Row align="middle" justify="space-between">
                <Typography.Title level={3}>Accepted Currencies</Typography.Title>
            </Row>
            <div className={b()}>
                {merchant?.supportedPaymentMethods.map((item) => (
                    <div className={b("option")} key={item.ticker}>
                        <Checkbox value={item.ticker} style={{lineHeight: "32px"}} checked={item.enabled}>
                            {item.displayName}
                        </Checkbox>
                        <Icon name={item.name.toLowerCase()} dir="tokens" className={b("icon")} />
                        {/* it's needed to prevent onClick on checkbox so as not to fire handler twice */}
                        <div
                            className={b("overlay")}
                            onClick={(e) => {
                                e.stopPropagation();
                                onChange(item.ticker);
                            }}
                        />
                    </div>
                ))}
                <SpinWithMask isLoading={supportedMethodsReqState.loading} />
            </div>
        </>
    );
};

export default PaymentMethodsSelect;
