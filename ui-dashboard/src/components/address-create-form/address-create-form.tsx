import * as React from "react";
import {Form, Input, Button, Space, Select, FormInstance} from "antd";
import {MerchantAddressParams, BLOCKCHAIN} from "src/types";
import {sleep} from "src/utils";

interface Props {
    onCancel: () => void;
    onFinish: (values: MerchantAddressParams, form: FormInstance<MerchantAddressParams>) => Promise<void>;
    isFormSubmitting: boolean;
}

const AddressCreateForm: React.FC<Props> = (props: Props) => {
    const [form] = Form.useForm<MerchantAddressParams>();

    const onSubmit = async (values: MerchantAddressParams) => {
        await props.onFinish(values, form);
    };

    const onCancel = async () => {
        props.onCancel();

        await sleep(1000);
        form.resetFields();
    };

    return (
        <Form<MerchantAddressParams> form={form} onFinish={onSubmit} layout="vertical">
            <div>
                <Form.Item
                    label="Blockchain"
                    name="blockchain"
                    rules={[{required: true, message: "Field is required"}]}
                >
                    <Select options={BLOCKCHAIN.map((item) => ({value: item, label: item}))} />
                </Form.Item>
                <Form.Item
                    label="Name"
                    name="name"
                    rules={[{required: true, message: "Field is required"}]}
                    validateTrigger="onBlur"
                >
                    <Input placeholder="My TrustWallet" />
                </Form.Item>
                <Form.Item
                    label="Blockchain Address"
                    name="address"
                    validateTrigger="onBlur"
                    rules={[{required: true, message: "Field is required"}]}
                >
                    <Input placeholder="0x9cEF7DE49...." />
                </Form.Item>
            </div>
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

export default AddressCreateForm;
