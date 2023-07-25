import * as React from "react";
import {Form, Input, Button, Space, Typography, FormInstance} from "antd";
import {WebhookSettings} from "src/types";
import {sleep} from "src/utils";
import LinkInput from "src/components/link-input/link-input";

interface Props {
    onCancel: (value: boolean) => void;
    onFinish: (values: WebhookSettings, form: FormInstance<WebhookSettings>) => Promise<void>;
    webhookSettings?: WebhookSettings;
    isFormSubmitting: boolean;
}

const linkPrefix = "https://";

const WebhookSettingsForm: React.FC<Props> = (props: Props) => {
    const [form] = Form.useForm<WebhookSettings>();

    React.useEffect(() => {
        if (props.webhookSettings) {
            form.setFieldsValue({...props.webhookSettings, url: props.webhookSettings.url.slice(8)});
        }
    }, [props.webhookSettings]);

    const onSubmit = async (values: WebhookSettings) => {
        values.url = linkPrefix + values.url;
        await props.onFinish(values, form);
    };

    const onCancel = async () => {
        props.onCancel(false);

        await sleep(1000);
        form.resetFields();
    };

    return (
        <Form<WebhookSettings> form={form} onFinish={onSubmit} layout="vertical">
            <div>
                <LinkInput label="Webhook URL" name="url" placeholder="your-store.com/hook" required />
                <Form.Item
                    label="Webhook Secret"
                    name="secret"
                    extra={
                        <>
                            <Typography.Paragraph>
                                <Typography.Text type="secondary">
                                    If provided, we will sign every webhook using HMAC, so you can verify request's
                                    authenticity
                                </Typography.Text>
                            </Typography.Paragraph>
                            <Typography.Paragraph>
                                <Typography.Link href="https://docs.o2pay.co/webhooks" target="_blank">
                                    Read more about webhook signatures
                                </Typography.Link>
                            </Typography.Paragraph>
                        </>
                    }
                >
                    <Input />
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

export default WebhookSettingsForm;
