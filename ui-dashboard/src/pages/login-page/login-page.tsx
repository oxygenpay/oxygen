import "./login-page.scss";

import * as React from "react";
import {AxiosError} from "axios";
import {useNavigate, useLocation} from "react-router-dom";
import {Modal, Button, Typography, Form, Input, notification} from "antd";
import {GoogleOutlined, CheckOutlined} from "@ant-design/icons";
import logoImg from "/fav/android-chrome-192x192.png";
import bevis from "src/utils/bevis";
import {useMount} from "react-use";
import localStorage from "src/utils/local-storage";
import {AuthProvider, UserCreateForm} from "src/types";
import authProvider from "src/providers/auth-provider";
import {sleep} from "src/utils";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";

const b = bevis("login-page");

interface LoginState {
    isNeedLogout: boolean;
}

const LoginPage: React.FC = () => {
    const [form] = Form.useForm<UserCreateForm>();
    const [api, contextHolder] = notification.useNotification();
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const [providersList, setProvidersList] = React.useState<AuthProvider[]>([]);
    const navigate = useNavigate();
    const state: LoginState = useLocation().state;

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
            navigate("/", {
                state: {realoadUserInfo: true}
            });
            openNotification("Welcome to the our community", "");

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const googleRedirectLink = (): string => {
        let host = import.meta.env.VITE_BACKEND_HOST;
        if (host == "//") {
            host = "";
        }

        return `${host}/api/dashboard/v1/auth/redirect`;
    };

    useMount(async () => {
        window.addEventListener("popstate", () => navigate("/login", {replace: true}));

        if (state?.isNeedLogout) {
            localStorage.remove("merchantId");
        } else {
            try {
                await authProvider.getCookie();
                await authProvider.getMe();
                navigate("/");
            } catch (e) {
                if (e instanceof AxiosError && e.response?.status === 401) {
                    localStorage.remove("merchantId");
                }
            }
        }

        const availProviders = await authProvider.getProviders();
        setProvidersList(availProviders ?? []);
    });

    const isLoading = providersList.length === 0;

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
                        <SpinWithMask isLoading={isLoading} />
                        {!isLoading ? (
                            <>
                                {providersList.findIndex((item) => item.name === "email") !== -1 ? (
                                    <Form<UserCreateForm>
                                        form={form}
                                        onFinish={onSubmit}
                                        layout="vertical"
                                        className={b("form")}
                                    >
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
                                ) : null}

                                {providersList.length == 2 ? (
                                    <Typography.Text className={b("text-or")}>OR</Typography.Text>
                                ) : null}

                                {providersList.findIndex((item) => item.name === "google") !== -1 ? (
                                    <Button
                                        key="submit"
                                        type="primary"
                                        href={googleRedirectLink()}
                                        className={b("btn")}
                                    >
                                        Sign in / Register with Google <GoogleOutlined />
                                    </Button>
                                ) : null}
                            </>
                        ) : null}
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
