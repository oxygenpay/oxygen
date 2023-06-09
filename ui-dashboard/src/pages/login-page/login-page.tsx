import "./login-page.scss";

import * as React from "react";
import {useNavigate} from "react-router-dom";
import {Modal, Button, Typography, Form, Input, notification} from "antd";
import {GoogleOutlined, CheckOutlined} from "@ant-design/icons";
import logoImg from "/fav/android-chrome-192x192.png";
import bevis from "src/utils/bevis";
import {useMount} from "react-use";
import localStorage from "src/utils/local-storage";
import {UserCreateForm} from "src/types";
import authProvider from "src/providers/auth-provider";
import {sleep} from "src/utils";

const b = bevis("login-page");

const LoginPage: React.FC = () => {
    const [form] = Form.useForm<UserCreateForm>();
    const [api, contextHolder] = notification.useNotification();
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const navigate = useNavigate();

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const onSubmit = async (values: UserCreateForm) => {
        try {
            setIsFormSubmitting(true);
            await authProvider.createUser(values);
            navigate("/");
            openNotification("Welcome to the our community", "");

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    useMount(() => {
        window.addEventListener("popstate", () => navigate("/login", {replace: true}));
        localStorage.remove("merchantId");
    });

    return (
        <>
            {contextHolder}
            <Modal
                title={
                    <>
                        <div className={b("logo")}>
                            <img src={logoImg} alt="logo" className={b("logo-img")} />
                            <Typography.Title className={b("logo-text")}>OxygenPay</Typography.Title>
                        </div>
                        <Typography.Title level={2}>Sign In 🔐</Typography.Title>
                        <Form<UserCreateForm> form={form} onFinish={onSubmit} layout="vertical" className={b("form")}>
                            <Form.Item
                                name="email"
                                rules={[
                                    {
                                        type: "email",
                                        message: "The input is not valid email"
                                    },
                                    {
                                        required: true,
                                        message: "Please input your email"
                                    }
                                ]}
                            >
                                <Input placeholder="Email" />
                            </Form.Item>
                            <Form.Item
                                name="password"
                                rules={[
                                    {
                                        required: true,
                                        message: "Please input your password"
                                    }
                                ]}
                            >
                                <Input.Password placeholder="Password" />
                            </Form.Item>
                            <Button
                                disabled={isFormSubmitting}
                                loading={isFormSubmitting}
                                type="primary"
                                htmlType="submit"
                                className={b("btn")}
                            >
                                Sign in
                            </Button>
                        </Form>
                        <Typography.Text className={b("text-or")}>OR</Typography.Text>
                        <Button
                            key="submit"
                            type="primary"
                            href={`${import.meta.env.VITE_BACKEND_HOST}/api/dashboard/v1/auth/redirect`}
                            className={b("btn")}
                        >
                            Sign in / Register with Google <GoogleOutlined />
                        </Button>
                    </>
                }
                maskStyle={{
                    backgroundColor: "#ffffff"
                }}
                centered
                open
                closable={false}
                footer={null}
            />
        </>
    );
};

export default LoginPage;
