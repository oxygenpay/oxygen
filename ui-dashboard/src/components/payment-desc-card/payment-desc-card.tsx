import "./payment-desc-card.scss";

import * as React from "react";
import {Descriptions, Tag} from "antd";
import {CopyOutlined} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import {Payment, CURRENCY_SYMBOL} from "src/types";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import copyToClipboard from "src/utils/copy-to-clipboard";
import TimeLabel from "src/components/time-label/time-label";
import renderStrippedStr from "src/utils/render-stripped-str";

interface Props {
    data?: Payment;
    openNotificationFunc: (title: string, description: string) => void;
}

const emptyState: Payment = {
    id: "loading",
    orderId: "loading",
    price: "",
    type: "payment",
    createdAt: "1997-05-01 15:00",
    status: "failed",
    currency: "USD",
    redirectUrl: "loading",
    paymentUrl: "loading",
    description: "loading",
    isTest: false
};

const b = bevis("payment-desc-card");

const PaymentDescCard: React.FC<Props> = ({data, openNotificationFunc}) => {
    React.useEffect(() => {
        if (!data) {
            data = emptyState;
        }
    }, [data]);

    return (
        <>
            <SpinWithMask isLoading={!data} />
            {data && (
                <>
                    <Descriptions>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>ID</span>}>
                            <span className={data.isTest ? b("test-label") : ""}>{data.id}</span>{" "}
                            {data.isTest && <Tag color="yellow">test payment</Tag>}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Status</span>}>
                            <PaymentStatusLabel status={data.status} />
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Created at</span>}>
                            <TimeLabel time={data.createdAt} />
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Order ID</span>}>
                            {data.orderId ?? "Not provided"}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Price</span>}>
                            {`${CURRENCY_SYMBOL[data.currency]}${data.price}`}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Description</span>}>
                            {data.description ?? "Not provided"}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Payment URL</span>}>
                            <span className={b("link__text")}>{renderStrippedStr(data?.paymentUrl ?? "")}</span>
                            {
                                <CopyOutlined
                                    className={b("link")}
                                    onClick={() =>
                                        copyToClipboard(data?.paymentUrl ? data.paymentUrl : "", openNotificationFunc)
                                    }
                                />
                            }
                        </Descriptions.Item>

                        {data.additionalInfo?.payment?.customerEmail ? (
                            <>
                                <Descriptions.Item span={3} label={<span className={b("item-title")}>Customer</span>}>
                                    {data.additionalInfo.payment.customerEmail}
                                </Descriptions.Item>
                            </>
                        ) : null}

                        {data.additionalInfo?.payment?.selectedCurrency ? (
                            <>
                                <Descriptions.Item
                                    span={3}
                                    label={<span className={b("item-title")}>Selected Payment Method</span>}
                                >
                                    {data.additionalInfo.payment.selectedCurrency}
                                </Descriptions.Item>
                            </>
                        ) : null}
                    </Descriptions>
                </>
            )}
        </>
    );
};

export default PaymentDescCard;
