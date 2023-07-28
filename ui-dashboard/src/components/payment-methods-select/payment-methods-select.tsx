import "./payment-methods-select.scss";

import * as React from "react";
import {useAsyncFn, useMount} from "react-use";
import {Row, Typography, Table, Result} from "antd";
import {ColumnsType} from "antd/es/table";
import {BlockchainTicker} from "src/types/index";
import merchantProvider from "src/providers/merchant-provider";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import useSharedMerchant from "src/hooks/use-merchant";
import PaymentMethodsItem from "src/components/payment-method-item/payment-method-item";

interface AvailableBlockchainsType {
    name: string;
}

const PaymentMethodsSelect: React.FC = () => {
    const {merchant, getMerchant} = useSharedMerchant();
    const {merchantId} = useSharedMerchantId();
    const [supportedMethodsReqState, updateSupportedMethods] = useAsyncFn(merchantProvider.updateSupportedMethods);
    const [availableBlockchains, setAvailableBlockchains] = React.useState<AvailableBlockchainsType[]>([]);
    const [isAvailableBlockchainsLoading, setIsAvailableBlockchainsLoading] = React.useState<boolean>(false);

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

    const paymentMethodsColumns: ColumnsType<AvailableBlockchainsType> = [
        {
            title: "Network",
            dataIndex: "network",
            key: "network",
            width: "min-content",
            render: (_, record) => <span style={{whiteSpace: "nowrap"}}>{record.name}</span>
        },
        {
            title: "Currencies",
            dataIndex: "currencies",
            key: "currencies",
            render: (_, record) => (
                <div>
                    <div key={record.name}>
                        <PaymentMethodsItem
                            title={record.name}
                            items={
                                merchant?.supportedPaymentMethods.filter(
                                    (paymentItem) => paymentItem.blockchainName === record.name
                                ) ?? []
                            }
                            onChange={onChange}
                            isLoading={supportedMethodsReqState.loading}
                        />
                    </div>
                </div>
            )
        }
    ];

    const getBlockchainsList = () => {
        if (!merchant) {
            return [];
        }

        return [...new Set(merchant.supportedPaymentMethods.map((item) => item.blockchainName))].map((item) => ({
            name: item
        }));
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
        setIsAvailableBlockchainsLoading(true);
        setAvailableBlockchains(getBlockchainsList());
        setIsAvailableBlockchainsLoading(false);
    }, [merchant?.supportedPaymentMethods]);

    return (
        <>
            <Row align="middle" justify="space-between">
                <Typography.Title level={3}>Accepted Currencies</Typography.Title>
            </Row>
            <Table
                columns={paymentMethodsColumns}
                dataSource={availableBlockchains}
                rowKey={(record) => record.name}
                loading={isAvailableBlockchainsLoading}
                pagination={false}
                size="middle"
                locale={{
                    emptyText: (
                        <Result
                            icon={null}
                            title="Accepted currencies will appear there after you add new type of supported currency"
                        ></Result>
                    )
                }}
            />
        </>
    );
};

export default PaymentMethodsSelect;
