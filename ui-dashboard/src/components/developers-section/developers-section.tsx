import * as React from "react";
import {Button, Row, Space, Typography} from "antd";
import {CopyOutlined, CodeOutlined} from "@ant-design/icons";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import copyToClipboard from "src/utils/copy-to-clipboard";

interface Props {
    openPopupFunc: (title: string, desc: string) => void;
}

const DevelopersSection: React.FC<Props> = (props: Props) => {
    const {merchantId} = useSharedMerchantId();

    return (
        <>
            <Row align="middle" justify="space-between">
                <Typography.Title level={3}>Developer section</Typography.Title>
            </Row>
            <Row align="middle">
                <Typography.Text>Merchant ID</Typography.Text>

                <Space
                    style={{cursor: "pointer", marginLeft: "10px"}}
                    onClick={() => copyToClipboard(merchantId!, props.openPopupFunc)}
                >
                    <Typography.Text code>{merchantId} </Typography.Text>
                    <CopyOutlined />
                </Space>

                <Space style={{marginLeft: "10px"}}>
                    <Button href={"https://docs.o2pay.co/specs/merchant/v1/"} target={"_blank"} icon={<CodeOutlined />}>
                        API Reference
                    </Button>
                </Space>
            </Row>
        </>
    );
};

export default DevelopersSection;
