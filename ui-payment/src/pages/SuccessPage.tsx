import * as React from "react";
import {useLocation, useNavigate} from "react-router-dom";
import {useMount} from "react-use";
import Icon from "src/components/Icon";
import {Payment} from "src/types";
import LoadingTextIcon from "src/components/LoadingTextIcon";
import renderConvertedResult from "src/utils/renderConvertedResult";
import renderCurrency from "src/utils/renderCurrency";

interface SuccessState {
    payment: Payment;
}

const SuccessPage: React.FC = () => {
    const [payment, setPayment] = React.useState<Payment>();
    const [isSuccessMessageVisible, setIsSuccessMessageVisible] = React.useState<boolean>(false);
    const state: SuccessState = useLocation().state;
    const navigate = useNavigate();

    useMount(() => {
        if (state?.payment) {
            setPayment(state.payment);
        } else {
            navigate("/not-found");
        }
    });

    const showMerchantMsg = () => {
        if (!payment?.paymentInfo || !payment.paymentInfo.successMessage) {
            return;
        }

        setIsSuccessMessageVisible(true);
    };

    const LinkToOurSiteAfterSuccess = "#";
    const successLink = payment?.paymentInfo?.successUrl ? payment.paymentInfo.successUrl : LinkToOurSiteAfterSuccess;
    const convertedResultRendered = renderConvertedResult(
        payment?.paymentInfo?.amountFormatted,
        payment?.paymentMethod?.ticker
    );

    return (
        <>
            {!payment && (
                <>
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon />
                </>
            )}

            {!isSuccessMessageVisible && payment && payment.customer && payment.paymentInfo && payment.paymentMethod && (
                <>
                    <div className="mx-auto h-16 w-16 flex items-center justify-center mb-2.5 sm:mb-2">
                        <div className="shrink-0">
                            <Icon name="store" className="h-16 w-16" />
                        </div>
                    </div>
                    <span
                        className={`block mx-auto text-2xl font-medium text-center ${
                            payment?.description ? "mb-1" : "mb-5"
                        }`}
                    >
                        {payment.merchantName}
                    </span>
                    <span className="block mx-auto text-sm font-medium text-card-desc text-center max-w-sm-desc-size mb-8 sm:mb-5">
                        {payment?.description || <i>No description provided</i>}
                    </span>
                    <div className="relative mb-8 sm:mb-8 after:absolute after:top-1/2 after:-translate-y-1/2 after:w-[calc(100%-32px)] after:h-px after:border-dashed after:border-[1px] after:my-auto after:border-main-green-3 after:left-1/2 after:-translate-x-1/2">
                        <div className="before:absolute before:-left-4 before:w-8 before:h-8 before:rounded-full before:my-auto before:bg-white before:top-1/2 before:-translate-y-1/2 after:absolute after:-right-4 after:w-8 after:h-8 after:rounded-full after:my-auto after:bg-white after:top-1/2 after:-translate-y-1/2">
                            <div className="w-full bg-[#ECF5F5] h-36 flex items-center justify-center">
                                <span className="block mx-auto text-[30px] leading-[30px] font-medium text-center text-main-green-1 sm:max-w-sm-desc-size">
                                    Payment successful!
                                </span>
                            </div>
                            <div className="bg-[#ECF5F5] h-28 flex items-center justify-center pb-5">
                                <div>
                                    <span className="block mx-auto text-[40px] leading-[40px] font-medium text-center m-4">
                                        {renderCurrency(payment.currency, payment.price)}
                                    </span>
                                    <span className="block mx-auto text-xl font-medium text-center">
                                        {convertedResultRendered ? convertedResultRendered : "loading.."}
                                    </span>
                                </div>
                            </div>
                        </div>
                    </div>
                </>
            )}

            {!isSuccessMessageVisible && payment?.paymentInfo?.successAction && (
                <div className="mx-auto flex items-center justify-center mb-3">
                    {payment.paymentInfo.successAction === "redirect" ? (
                        <a
                            className={`relative border rounded-3xl bg-main-green-1 border-main-green-1 h-14 font-medium text-xl text-white flex items-center justify-center basis-full`}
                            href={successLink}
                        >
                            Back to site
                            <Icon
                                name="arrow_right_white"
                                className="absolute h-5 w-5 right-16 xs:right-12 md:right-20"
                            />
                        </a>
                    ) : (
                        <button
                            className={`relative border rounded-3xl bg-main-green-1 border-main-green-1 h-14 font-medium text-xl text-white flex items-center justify-center basis-full`}
                            onClick={() => showMerchantMsg()}
                        >
                            See message
                            <Icon
                                name="arrow_right_white"
                                className="absolute h-5 w-5 right-16 xs:right-8 md:right-15"
                            />
                        </button>
                    )}
                </div>
            )}

            {isSuccessMessageVisible && payment && payment?.paymentInfo?.successMessage && (
                <>
                    <div className="mx-auto h-16 w-16 flex items-center justify-center mb-2.5 sm:mb-2">
                        <div className="shrink-0">
                            <Icon name="store" className="h-16 w-16" />
                        </div>
                    </div>
                    <span
                        className={`block mx-auto text-2xl font-medium text-center ${
                            payment?.description ? "mb-1" : "mb-5"
                        }`}
                    >
                        Message from <br /> {payment.merchantName}:
                    </span>
                    <span className="block text-lg font-medium text-card-desc">
                        {payment.paymentInfo.successMessage}
                    </span>
                </>
            )}
        </>
    );
};

export default SuccessPage;
