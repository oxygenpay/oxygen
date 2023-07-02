import "./App.css";

import "tw-elements";
import React from "react";
import {useMount} from "react-use";
import {Routes, Route, useNavigate, useLocation} from "react-router-dom";
import SuccessPage from "src/pages/SuccessPage";
import PaymentPage from "src/pages/PaymentPage";
import NotFoundPage from "src/pages/NotFoundPage";
import ErrorPage from "src/pages/ErrorPage";
import PaymentContext from "src/hooks/paymentContext";
import PaymentLinkContext from "src/hooks/linkContext";
import paymentProvider from "src/providers/paymentProvider";
import {Payment, PaymentLink} from "src/types";
import Icon from "src/components/Icon";
import LinkPage from "src/pages/LinkPage";
import {toggled} from "./providers/toggles";

const App: React.FC = () => {
    const navigate = useNavigate();
    const location = useLocation();
    const id = React.useRef(location.pathname.match(/\/([^/]+)$/)?.[1]);

    const [payment, setPayment] = React.useState<Payment>();
    const [paymentLink, setPaymentLink] = React.useState<PaymentLink>();

    const getPayment = async () => {
        if (!id.current || id.current === "not-found") {
            return;
        }
        try {
            const payment = await paymentProvider.getPayment(id.current);
            setPayment(payment);
        } catch {
            navigate(`/error/${id.current}`, {
                state: {
                    payment
                }
            });
        }
    };

    const getPaymentLink = async () => {
        if (!id.current || id.current === "not-found") {
            return;
        }
        try {
            const paymentLink = await paymentProvider.getPaymentLink(id.current);
            setPaymentLink(paymentLink);
        } catch {
            navigate("/not-found");
        }
    };

    useMount(async () => {
        try {
            await paymentProvider.getCSRFCookie();

            if (location.pathname.startsWith("/link/")) {
                await getPaymentLink();
            } else {
                await getPayment();
            }
        } catch {
            console.error("Error ocurred");
        }
    });

    React.useEffect(() => {
        if (payment) {
            document.title = `O2Pay Payment: ${payment.merchantName}`;
        }
    }, [payment]);

    React.useEffect(() => {
        if (paymentLink) {
            document.title = `O2Pay Payment Link: ${paymentLink.merchantName}`;
        }
    }, [paymentLink]);

    return (
        <PaymentContext.Provider value={{payment, setPayment}}>
            <PaymentLinkContext.Provider value={{paymentLink, setPaymentLink}}>
                <main className="min-h-screen bg-main-green-3">
                    <div className="wrapper container mx-auto py-8 sm:pt-8 pb-0 relative">
                        <Icon
                            name="logo"
                            className="absolute m-auto left-0 right-0 top-[59px] sm:top-3.5 w-32 h-6 sm:w-44 sm:h-8"
                        />
                        <div className="sm:h-mobile-card-height sm:min-h-mobile-card flex flex-row justify-center mt-[4.4rem] sm:mt-7">
                            <div className="bg-white lg:w-[370px] xs:w-full md:w-[390px] max-w-md lg:rounded-[30px] sm:rounded-t-[30px] shadow-md p-[34px] xs:pt-4">
                                <Routes>
                                    <Route path="not-found/:id" element={<NotFoundPage />} />
                                    <Route path="success/:id" element={<SuccessPage />} />
                                    <Route path="pay/:id" element={<PaymentPage />} />
                                    <Route path="error/:id" element={<ErrorPage />} />
                                    <Route path="link/:id" element={<LinkPage />} />
                                    <Route path="*" element={<NotFoundPage />} />
                                </Routes>
                            </div>
                        </div>
                        {toggled("show_branding") && (
                            <div className="pt-2 pb-4 text-sm sm:hidden">
                                <p className="text-center text-gray-500">
                                    Powered by self-hosted{" "}
                                    <a className="color-oxygen" target="_blank" href="https://o2pay.co">
                                        OxygenPay
                                    </a>
                                </p>
                            </div>
                        )}
                    </div>
                </main>
            </PaymentLinkContext.Provider>
        </PaymentContext.Provider>
    );
};

export default App;
