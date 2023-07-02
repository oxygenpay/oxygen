import * as React from "react";
import {Descriptions} from "antd";
import {CopyOutlined} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import {PaymentLink, CURRENCY_SYMBOL} from "src/types";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import copyToClipboard from "src/utils/copy-to-clipboard";
import TimeLabel from "src/components/time-label/time-label";

interface Props {
    data?: PaymentLink;
    openNotificationFunc: (title: string, description: string) => void;
}

const emptyState: PaymentLink = {
    id: "loading",
    price: 0,
    successAction: "redirect",
    createdAt: "loading",
    currency: "USD",
    name: "loading",
    url: "loading",
    redirectUrl: "loading",
    description: "loading",
    successMessage: "loading"
};

const b = bevis("payment-desc-card");

const PaymentLinkDescCard: React.FC<Props> = ({data, openNotificationFunc}) => {
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
                            {data.id}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Created at</span>}>
                            <TimeLabel time={data.createdAt} />
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Price</span>}>
                            {`${CURRENCY_SYMBOL[data.currency]}${data.price}`}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Description</span>}>
                            {data.description ?? "Not provided"}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>URL</span>}>
                            <span className={b("link__text")}>{data.url}</span>
                            <CopyOutlined
                                className={b("link")}
                                onClick={() => copyToClipboard(data?.url ?? "", openNotificationFunc)}
                            />
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Action</span>}>
                            {data.successAction === "redirect" ? "Redirect" : "Show message"}
                        </Descriptions.Item>

                        {data.successAction === "redirect" ? (
                            <>
                                <Descriptions.Item
                                    span={3}
                                    label={<span className={b("item-title")}>Redirect URL</span>}
                                >
                                    {data.redirectUrl ?? "Not provided"}
                                </Descriptions.Item>
                            </>
                        ) : (
                            <Descriptions.Item
                                span={3}
                                label={<span className={b("item-title")}>Success Message</span>}
                            >
                                {data.successMessage ?? "Not provided"}
                            </Descriptions.Item>
                        )}
                    </Descriptions>
                </>
            )}
        </>
    );
};

export default PaymentLinkDescCard;
