import * as React from "react";
import bevis from "src/utils/bevis";
import {Checkbox, Row, Typography} from "antd";
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
        <>
            <Row align="middle" justify="space-between">
                <Typography.Title level={5}>{props.title}</Typography.Title>
            </Row>
            <div className={b()}>
                {props.items.map((item) => (
                    <div className={b("option")} key={item.displayName}>
                        <Checkbox value={item.ticker} style={{lineHeight: "32px"}} checked={item.enabled}>
                            {item.displayName}
                        </Checkbox>
                        <Icon name={item.name.toLowerCase()} dir="crypto" className={b("icon")} />
                        {/* it's needed to prevent onClick on checkbox so as not to fire handler twice */}
                        <div
                            className={b("overlay")}
                            onClick={(e) => {
                                e.stopPropagation();
                                props.onChange(item.ticker);
                            }}
                        />
                        <SpinWithMask isLoading={props.isLoading} />
                    </div>
                ))}
            </div>
        </>
    );
};

export default PaymentMethodsItem;
