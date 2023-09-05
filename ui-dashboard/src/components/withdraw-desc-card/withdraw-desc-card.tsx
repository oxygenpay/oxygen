import "./withdraw-desc-card.scss";

import * as React from "react";
import {Descriptions, Tag, Row, Col, Statistic} from "antd";
import {CopyOutlined, LinkOutlined} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import {Payment, MerchantBalance, MerchantAddress} from "src/types";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import copyToClipboard from "src/utils/copy-to-clipboard";
import {useMount} from "react-use";
import TimeLabel from "src/components/time-label/time-label";
import renderStrippedStr from "src/utils/render-stripped-str";

interface Props {
    data?: Payment;
    balances: MerchantBalance[];
    addresses: MerchantAddress[];
    openNotificationFunc: (title: string, description: string) => void;
}

const emptyState: Payment = {
    id: "loading",
    orderId: "loading",
    price: "",
    type: "payment",
    createdAt: "loading",
    status: "failed",
    currency: "USD",
    redirectUrl: "loading",
    paymentUrl: "loading",
    description: "loading",
    isTest: false
};

const b = bevis("withdraw-desc-card");

const WithdrawalDescCard: React.FC<Props> = ({data, openNotificationFunc, addresses, balances}) => {
    const [address, setAddress] = React.useState<MerchantAddress>();
    const [balance, setBalance] = React.useState<MerchantBalance>();

    const loadAllParams = async () => {
        if (!data) {
            data = emptyState;
        } else {
            setAddress(addresses.find((item) => item.id === data?.additionalInfo?.withdrawal?.addressId));
            setBalance(balances.find((item) => item.id === data?.additionalInfo?.withdrawal?.balanceId));
        }
    };

    useMount(() => {
        loadAllParams();
    });

    React.useEffect(() => {
        loadAllParams();
    }, [data]);

    return (
        <>
            <SpinWithMask isLoading={!data || !balance} />
            {data?.additionalInfo?.withdrawal && balance && (
                <>
                    <Descriptions>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Balance</span>}>
                            <span
                                className={data.isTest ? b("test-label") : ""}
                            >{`${balance.blockchainName} ${balance.currency}`}</span>
                            {data.isTest && <Tag color="yellow">test balance</Tag>}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Status</span>}>
                            <PaymentStatusLabel status={data.status} />
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Created at</span>}>
                            <TimeLabel time={data.createdAt} />
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Recipient</span>}>
                            {address && <span>{address.name}</span>}

                            {!address && <span>The address had been deleted</span>}
                        </Descriptions.Item>
                        {address && (
                            <Descriptions.Item span={3}>
                                <>
                                    {address.address}{" "}
                                    <CopyOutlined
                                        className={b("copy-btn")}
                                        onClick={() => copyToClipboard(address.address, openNotificationFunc)}
                                    />
                                </>
                            </Descriptions.Item>
                        )}
                        <Descriptions.Item span={3}>
                            <Row justify="space-between" gutter={[90, 10]}>
                                <Col>
                                    <Statistic title="Amount" value={`${data.price} ${data.currency}`} />
                                </Col>
                                <Col>
                                    <Statistic
                                        title="Service fee"
                                        value={`${data.additionalInfo?.withdrawal?.serviceFee} ${data.currency}`}
                                    />
                                </Col>
                            </Row>
                        </Descriptions.Item>

                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Transaction ID</span>}>
                            {data.additionalInfo.withdrawal.transactionHash != null
                                ? renderStrippedStr(data.additionalInfo.withdrawal.transactionHash, 24, -12)
                                : "Transaction hash is not created yet"}
                            {data.additionalInfo.withdrawal.transactionHash != null ? " " : null}
                            {data.additionalInfo.withdrawal.transactionHash != null ? (
                                <CopyOutlined
                                    className={b("copy-btn")}
                                    onClick={() =>
                                        copyToClipboard(
                                            data?.additionalInfo?.withdrawal?.transactionHash
                                                ? data.additionalInfo.withdrawal.transactionHash
                                                : "",
                                            openNotificationFunc
                                        )
                                    }
                                />
                            ) : null}
                        </Descriptions.Item>
                        {data.additionalInfo?.payment?.customerEmail ? (
                            <>
                                <Descriptions.Item span={2} label={<span className={b("item-title")}>Customer</span>}>
                                    {data.additionalInfo.payment.customerEmail}
                                </Descriptions.Item>
                            </>
                        ) : null}
                        {data.additionalInfo?.payment?.selectedCurrency ? (
                            <>
                                <Descriptions.Item
                                    span={2}
                                    label={<span className={b("item-title")}>Selected Payment Method</span>}
                                >
                                    {data.additionalInfo.payment.selectedCurrency}
                                </Descriptions.Item>
                            </>
                        ) : null}
                        {data.additionalInfo.withdrawal.explorerLink != null ? (
                            <Descriptions.Item>
                                <a
                                    href={data.additionalInfo.withdrawal.explorerLink ?? ""}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                >
                                    View on blockchain explorer <LinkOutlined className={b("copy-btn")} />
                                </a>
                            </Descriptions.Item>
                        ) : null}
                    </Descriptions>
                </>
            )}
        </>
    );
};

export default WithdrawalDescCard;
