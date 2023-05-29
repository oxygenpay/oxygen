import * as React from "react";
import Icon from "src/components/Icon";
import {useNavigate, useLocation} from "react-router-dom";
import LoadingTextIcon from "src/components/LoadingTextIcon";
import {Payment} from "src/types";

interface ErrorState {
    payment: Payment;
}

const ErrorPage: React.FC = () => {
    const state: ErrorState = useLocation().state;
    const [payment, setPayment] = React.useState<Payment>();
    const navigate = useNavigate();

    React.useEffect(() => {
        if (state?.payment) {
            setPayment(state.payment);
        } else {
            navigate("/not-found");
        }
    }, []);

    return (
        <>
            {!payment && (
                <>
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon />
                </>
            )}

            <>
                {payment && (
                    <>
                        <div className="mx-auto h-16 w-16 flex items-center justify-center mb-3.5 sm:mb-2">
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
                        <div className="mx-auto h-30 w-30 flex items-center justify-center mb-3 sm:mb-4">
                            <div className="shrink-0 justify-self-center">
                                <Icon name="error" className="h-30 w-30" />
                            </div>
                        </div>
                        <span className="block mx-auto text-3xl font-medium text-center mb-6 text-card-error">
                            Something went <br /> wrong
                        </span>
                        <span className="block mx-auto font-medium text-center text-card-desc text-sm max-w-xl-desc-size mb-6 sm:mb-[3.75rem]">
                            Payment ID <br /> {payment.id}
                        </span>
                        <div className="mx-auto flex items-center justify-center mb-3">
                            <a
                                className="relative border rounded-3xl border-main-green-1 w-full h-14 color-black font-medium text-xl flex items-center justify-center"
                                href={import.meta.env.VITE_SUPPORT_EMAIL}
                            >
                                Contact Support
                                <Icon
                                    name="arrow_right_black"
                                    className="h-5 w-5 absolute right-10 xs:right-4 md:right-14"
                                />
                            </a>
                        </div>
                    </>
                )}
            </>
        </>
    );
};

export default ErrorPage;
