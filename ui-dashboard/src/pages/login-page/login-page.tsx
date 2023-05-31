import "./login-page.scss";

import * as React from "react";
import {useNavigate} from "react-router-dom";
import {Modal, Button, Typography} from "antd";
import {GoogleOutlined} from "@ant-design/icons";
import logoImg from "/fav/android-chrome-192x192.png";
import bevis from "src/utils/bevis";
import {useMount} from "react-use";
import localStorage from "src/utils/local-storage";

const b = bevis("login-page");

const LoginPage: React.FC = () => {
    const navigate = useNavigate();

    useMount(() => {
        window.addEventListener("popstate", () => navigate("/login", {replace: true}));
        localStorage.remove("merchantId");
    });

    return (
        <>
            <Modal
                title={
                    <>
                        <div className={b("logo")}>
                            <img src={logoImg} alt="logo" className={b("logo-img")} />
                            <Typography.Title className={b("logo-text")}>OxygenPay</Typography.Title>
                        </div>
                        <Typography.Title level={2}>Sign In 🔐</Typography.Title>
                        <Button
                            key="submit"
                            type="primary"
                            href={`${import.meta.env.VITE_BACKEND_HOST}/api/dashboard/v1/auth/redirect`}
                            className={b("btn")}
                        >
                            Sign in with Google <GoogleOutlined />
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
