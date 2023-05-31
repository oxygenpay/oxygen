import * as React from "react";
import {Form, Input, Button, Space, FormInstance} from "antd";
import {MerchantAddress} from "src/types";
import {sleep} from "src/utils";

interface Props {
    onCancel: () => void;
    onFinish: (value: MerchantAddress, form: FormInstance<FormFields>) => Promise<void>;
    selectedAddress?: MerchantAddress;
    isFormSubmitting: boolean;
}

interface FormFields {
    name: string;
}

const AddressEditForm: React.FC<Props> = (props: Props) => {
    const [form] = Form.useForm<FormFields>();
    const [address, changeAddress] = React.useState<string>("");

    React.useEffect(() => {
        if (props.selectedAddress) {
            form.setFieldsValue({name: props.selectedAddress.name});
            changeAddress(props.selectedAddress.name);
        } else {
            changeAddress("");
        }
    }, [props.selectedAddress]);

    const onSubmit = async (values: FormFields) => {
        if (!props.selectedAddress) {
            return;
        }

        props.selectedAddress.name = values.name;
        await props.onFinish(props.selectedAddress, form);
    };

    const onCancel = async () => {
        props.onCancel();

        await sleep(1000);
        form.resetFields();
    };

    return (
        <Form<FormFields> form={form} onFinish={onSubmit} layout="vertical">
            <div>
                <Form.Item
                    label="Address name"
                    name="name"
                    rules={[{required: true, message: "Field is required"}]}
                    validateTrigger="onBlur"
                >
                    <Input onChange={(e) => changeAddress(e.target.value)} />
                </Form.Item>
            </div>
            <Space>
                <Button
                    loading={props.isFormSubmitting}
                    type="primary"
                    htmlType="submit"
                    disabled={!address || props.isFormSubmitting}
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

export default AddressEditForm;
