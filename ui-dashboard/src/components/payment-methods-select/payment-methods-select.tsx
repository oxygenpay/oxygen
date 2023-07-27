import "./payment-methods-select.scss";

import * as React from "react";
import {useAsyncFn, useMount} from "react-use";
import {Row, Typography} from "antd";
import {BlockchainTicker} from "src/types/index";
import merchantProvider from "src/providers/merchant-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import useSharedMerchant from "src/hooks/use-merchant";
import PaymentMethodsItem from "src/components/payment-method-item/payment-method-item";

const PaymentMethodsSelect: React.FC = () => {
    const {merchant, getMerchant} = useSharedMerchant();
    const {merchantId} = useSharedMerchantId();
    const [supportedMethodsReqState, updateSupportedMethods] = useAsyncFn(merchantProvider.updateSupportedMethods);
    const [availableBlockchains, setAvailableBlockchains] = React.useState<string[]>([]);

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

    const getBlockchainsList = () => {
        if (!merchant) {
            return [];
        }

        return [...new Set(merchant.supportedPaymentMethods.map((item) => item.blockchainName))];
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

    React.useEffect(() => {
        setAvailableBlockchains(getBlockchainsList());
    }, [merchant?.supportedPaymentMethods]);

    return (
        <>
            <Row align="middle" justify="space-between">
                <Typography.Title level={3}>Accepted Currencies</Typography.Title>
            </Row>
            <div>
                {availableBlockchains.map((item) => (
                    <div key={item}>
                        <PaymentMethodsItem
                            title={item}
                            items={
                                merchant?.supportedPaymentMethods.filter(
                                    (paymentItem) => paymentItem.blockchainName === item
                                ) ?? []
                            }
                            onChange={onChange}
                            isLoading={supportedMethodsReqState.loading}
                        />
                    </div>
                ))}
            </div>
        </>
    );
};

export default PaymentMethodsSelect;
