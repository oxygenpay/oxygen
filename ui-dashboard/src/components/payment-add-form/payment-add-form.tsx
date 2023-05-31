import * as React from "react";
import {v4 as uuidv4} from "uuid";
import {Form, Input, Button, Space, Select, InputNumber, Checkbox, Typography, FormInstance} from "antd";
import {PaymentParams, CURRENCY} from "src/types";
import {sleep} from "src/utils";
import LinkInput from "src/components/link-input/link-input";

interface Props {
    onCancel: () => void;
    onFinish: (values: PaymentParams, form: FormInstance<PaymentParams>) => Promise<void>;
    isFormSubmitting: boolean;
}

const linkPrefix = "https://";

const PaymentForm: React.FC<Props> = (props: Props) => {
    const [form] = Form.useForm<PaymentParams>();

    const onSubmit = async (values: PaymentParams) => {
        if (values.redirectUrl) {
            values.redirectUrl = linkPrefix + values.redirectUrl;
        } else {
            values.redirectUrl = undefined;
        }

        await props.onFinish(values, form);
    };

    const onCancel = async () => {
        props.onCancel();

        await sleep(1000);
        form.resetFields();
    };

    const minPrice = 0;
    const maxPrice = 10 ** 7;

    return (
        <Form<PaymentParams> form={form} initialValues={{id: uuidv4()}} onFinish={onSubmit} layout="vertical">
            <Form.Item name="id" hidden>
                <Input />
            </Form.Item>
            <Space>
                <Form.Item
                    label="Price"
                    name="price"
                    required
                    rules={[
                        {required: true, message: "Field is required"},
                        {
                            type: "number",
                            message: "Incorrect number value"
                        }
                    ]}
                    validateFirst
                    validateTrigger={["onChange", "onBlur"]}
                >
                    <InputNumber style={{width: "100%"}} precision={2} min={minPrice} max={maxPrice} />
                </Form.Item>
                <Form.Item
                    name="currency"
                    required
                    rules={[{required: true, message: "Field is required"}]}
                    style={{width: 80, marginTop: "30px"}}
                    initialValue="USD"
                >
                    <Select
                        style={{width: 80}}
                        options={CURRENCY.map((currency) => ({value: currency, label: currency}))}
                    />
                </Form.Item>
            </Space>
            <Form.Item
                label="Order ID"
                name="orderId"
                extra={
                    <Typography.Text type="secondary" style={{paddingBottom: "8px"}}>
                        Optional order id inside your system
                    </Typography.Text>
                }
            >
                <Input placeholder="orders#abc123" />
            </Form.Item>
            <Form.Item label="Description" name="description" style={{width: 300}}>
                <Input.TextArea placeholder="Your description" rows={2} />
            </Form.Item>
            <LinkInput
                placeholder="your-store.com/success"
                label="Customer redirect URL"
                name="redirectUrl"
                required={false}
            />
            <Form.Item name="isTest" style={{marginBottom: "0px"}} valuePropName="checked">
                <Checkbox>Test payment</Checkbox>
            </Form.Item>
            <Typography.Text type="secondary" style={{display: "block", marginBottom: "24px"}}>
                Test payments are processed using testnets like Ethereum Goerli
            </Typography.Text>
            <Space>
                <Button
                    disabled={props.isFormSubmitting}
                    loading={props.isFormSubmitting}
                    type="primary"
                    htmlType="submit"
                >
                    Save
                </Button>
                <Button danger onClick={onCancel}>
                    Cancel
                </Button>
            </Space>
        </Form>
    );
};

export default PaymentForm;
