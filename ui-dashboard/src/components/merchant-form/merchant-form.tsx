import * as React from "react";
import {Form, Input, Button, Space, FormInstance} from "antd";
import {Merchant, MerchantBase} from "src/types";
import {sleep} from "src/utils";
import LinkInput from "../link-input/link-input";

interface Props {
    activeMerchant?: Merchant;
    onCancel: () => void;
    onFinish: (values: MerchantBase, form: FormInstance<MerchantBase>) => Promise<void>;
    isFormSubmitting: boolean;
}

const linkPrefix = "https://";

const MerchantForm: React.FC<Props> = (props: Props) => {
    const [form] = Form.useForm<MerchantBase>();

    React.useEffect(() => {
        if (props.activeMerchant) {
            props.activeMerchant.website = props.activeMerchant.website.slice(8);
            form.setFieldsValue(props.activeMerchant);
        }
    }, [props.activeMerchant]);

    const onSubmit = async (values: MerchantBase) => {
        values.website = linkPrefix + values.website;
        await props.onFinish(values, form);
    };

    const onCancel = async () => {
        props.onCancel();

        await sleep(1000);
        form.resetFields();
    };

    return (
        <Form<MerchantBase> form={form} onFinish={onSubmit} layout="vertical">
            <div>
                <Form.Item
                    label="Store name"
                    name="name"
                    rules={[{required: true, message: "Field is required"}]}
                    validateTrigger="onBlur"
                >
                    <Input />
                </Form.Item>
                <LinkInput label="Website" name="website" placeholder="your-store.com" required />
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

export default MerchantForm;
