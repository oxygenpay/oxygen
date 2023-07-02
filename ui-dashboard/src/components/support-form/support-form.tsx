import * as React from "react";
import {Form, Input, Button, Space, Select, FormInstance} from "antd";
import {SupportMessage} from "src/types";
import {sleep} from "src/utils";

interface Props {
    onCancel: () => void;
    onFinish: (values: SupportMessage, form: FormInstance<SupportMessage>) => Promise<void>;
    isFormSubmitting: boolean;
}

const SupportForm: React.FC<Props> = (props: Props) => {
    const [form] = Form.useForm<SupportMessage>();

    const onSubmit = async (values: SupportMessage) => {
        await props.onFinish(values, form);
    };

    const onCancel = async () => {
        props.onCancel();

        await sleep(1000);
        form.resetFields();
    };

    return (
        <Form<SupportMessage> form={form} onFinish={onSubmit} layout="vertical">
            <div>
                <Form.Item
                    label="Topic"
                    name="topic"
                    required
                    rules={[{required: true, message: "Field is required"}]}
                    initialValue={{
                        value: "Support",
                        label: "Support"
                    }}
                    valuePropName="option"
                    style={{width: "150px"}}
                >
                    <Select
                        options={[
                            {
                                value: "Support",
                                label: "Support"
                            },
                            {
                                value: "Feedback",
                                label: "Feedback"
                            }
                        ]}
                    />
                </Form.Item>
                <Form.Item
                    label="Message"
                    name="message"
                    rules={[{required: true, message: "Field is required"}]}
                    validateTrigger="onBlur"
                >
                    <Input.TextArea spellCheck={false} autoSize={{minRows: 4, maxRows: 12}} />
                </Form.Item>
            </div>
            <Space>
                <Button
                    disabled={props.isFormSubmitting}
                    loading={props.isFormSubmitting}
                    type="primary"
                    htmlType="submit"
                >
                    Send
                </Button>
                <Button danger onClick={onCancel}>
                    Cancel
                </Button>
            </Space>
        </Form>
    );
};

export default SupportForm;
