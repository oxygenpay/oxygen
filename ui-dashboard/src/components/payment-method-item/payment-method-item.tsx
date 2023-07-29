import * as React from "react";
import bevis from "src/utils/bevis";
import {Space, Tag} from "antd";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import {BlockchainTicker, PaymentMethod} from "src/types/index";
import Icon from "src/components/icon/icon";

const b = bevis("payment-methods-select");

interface Props {
    title: string;
    items: PaymentMethod[];
    onChange: (ticker: BlockchainTicker) => void;
    isLoading: boolean;
}

const PaymentMethodsItem: React.FC<Props> = (props: Props) => {
    return (
        <Space size={[0, 8]} wrap className={b()}>
            {props.items.map((item) => (
                <Tag
                    key={item.name}
                    icon={<Icon name={item.name.toLowerCase()} dir="crypto" className={b("icon")} />}
                    color={item.enabled ? "#1777ff" : "#bebebe"}
                    className={b("option")}
                >
                    <span className={b("option-text")}>{item.name}</span>
                    <div
                        className={b("overlay")}
                        onClick={(e) => {
                            e.stopPropagation();
                            props.onChange(item.ticker);
                        }}
                    />
                    <SpinWithMask isLoading={props.isLoading} />
                </Tag>
            ))}
        </Space>
    );
};

export default PaymentMethodsItem;
