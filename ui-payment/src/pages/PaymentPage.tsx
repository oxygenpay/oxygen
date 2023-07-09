import * as React from "react";
import * as Yup from "yup";
import {useFormik} from "formik";
import {useNavigate} from "react-router-dom";
import {QRCodeSVG} from "qrcode.react";
import Icon from "src/components/Icon";
import {usePayment} from "src/hooks/paymentContext";
import Input from "src/components/Input";
import {CurrencyConvertResult, PaymentMethod} from "src/types";
import paymentProvider from "src/providers/paymentProvider";
import LoadingTextIcon from "src/components/LoadingTextIcon";
import CopyButton from "src/components/CopyButton";
import ProgressCircle from "src/components/ProgressCircle";
import DropDown, {DropDownItem} from "src/components/DropDown";
import renderConvertedResult from "src/utils/renderConvertedResult";

const schema = Yup.object({
    email: Yup.string().email().required("Please fill an email")
});

interface EmailForm {
    email: string;
}

const PaymentPage: React.FC = () => {
    const navigate = useNavigate();
    const {payment, setPayment} = usePayment();
    const [paymentProcessError, setPaymentProcessError] = React.useState<boolean>(false);
    const [emailFilled, setEmailFilled] = React.useState<boolean>(false);
    const [currencyChosen, setCurrencyChosen] = React.useState<boolean>(false);
    const [paymentMethod, setPaymentMethod] = React.useState<PaymentMethod>();
    const [convertResult, setConvertResult] = React.useState<CurrencyConvertResult>();
    const [availableMethods, setAvailableMethods] = React.useState<PaymentMethod[]>([]);

    const updatePayment = async () => {
        if (!payment?.id) {
            return;
        }

        try {
            const paymentResponce = await paymentProvider.getPayment(payment.id);
            setPayment(paymentResponce);
        } catch (error) {
            setPaymentProcessError(true);
            console.error("Error ocurred:", error);
        }
    };

    const formikConfig = useFormik({
        initialValues: {
            email: ""
        },
        onSubmit: async () => {
            if (!payment || !paymentMethod || !emailFilled || !currencyChosen) {
                return;
            }

            try {
                await paymentProvider.putPayment(payment.id);
                await updatePayment();
            } catch (error) {
                console.error("Error occurred:", error);
            }
        },
        validationSchema: schema
    });

    const getCryptoCurrencyConvert = async (params: {cryptoCurrency: string}) => {
        if (!payment) {
            return;
        }

        try {
            const response = await paymentProvider.currencyConvert({
                fiatCurrency: payment.currency,
                fiatAmount: String(payment.price),
                cryptoCurrency: params.cryptoCurrency
            });

            setConvertResult(response);
        } catch (error) {
            console.error("Error ocurred:", error);
        }
    };

    const getSupportedMethods = async () => {
        if (!payment?.id || availableMethods.length > 0) {
            return;
        }

        if (payment.paymentMethod) {
            setPaymentMethod(payment.paymentMethod);

            try {
                await getCryptoCurrencyConvert({cryptoCurrency: payment.paymentMethod.ticker});
                setCurrencyChosen(true);
            } catch (error) {
                console.error("Error ocurred:", error);
                setCurrencyChosen(false);
            }
        }

        if (payment.customer) {
            formikConfig.resetForm({values: {email: payment.customer.email}});
            setEmailFilled(true);
        }

        try {
            const supportedMethods = await paymentProvider.getSupportedMethods(payment.id);
            setAvailableMethods(supportedMethods.availableMethods);
            setPayment(payment);
        } catch (error) {
            setCurrencyChosen(false);
            setEmailFilled(false);
            console.error("Error ocurred:", error);
        }
    };

    React.useEffect(() => {
        if (!payment?.id) {
            return;
        }

        if (payment.paymentInfo?.status === "failed") {
            navigate(`/error/${payment.id}`, {
                state: {
                    payment
                }
            });
        } else if (payment.isLocked && payment.paymentInfo?.status === "pending") {
            setTimeout(updatePayment, 2000);
        } else if (
            payment.isLocked &&
            (payment.paymentInfo?.status === "success" || payment.paymentInfo?.status === "inProgress")
        ) {
            navigate(`/success/${payment.id}`, {
                state: {
                    payment
                }
            });
        } else {
            getSupportedMethods();
        }
    }, [payment]);

    const onSelectPaymentMethod = async (cryptoCurrency: string) => {
        if (
            !payment ||
            cryptoCurrency === paymentMethod?.name ||
            (!paymentMethod && payment.paymentMethod?.ticker === cryptoCurrency)
        ) {
            return;
        }

        const selectedMethod = availableMethods.find((availableMethod) => availableMethod.ticker === cryptoCurrency);
        setPaymentMethod(selectedMethod);
        await getCryptoCurrencyConvert({cryptoCurrency});

        if (!selectedMethod) {
            return;
        }

        try {
            await paymentProvider.setPaymentMethod(payment.id, selectedMethod);
            setCurrencyChosen(true);
        } catch (error) {
            setCurrencyChosen(false);
            console.error("Error ocurred:", error);
        }
    };

    const checkCustomer = async (e: React.FocusEvent<string, Element>, email: string) => {
        formikConfig.handleBlur(e);

        const error = formikConfig.errors["email"];

        if (!payment?.id || !email || error) {
            setEmailFilled(false);
            return;
        }

        try {
            await paymentProvider.setCustomer(payment.id, {email});
            setEmailFilled(true);
        } catch (error) {
            setEmailFilled(false);
            console.error("Error ocurred:", error);
        }
    };

    const getPrice = () => {
        if (payment !== undefined) {
            if (payment.currency === undefined || payment.price === undefined) {
                return;
            }

            if (payment.currency === "USD") {
                return `$${payment.price.toFixed(2)}`;
            }
            if (payment.currency === "EUR") {
                return `€${payment.price.toFixed(2)}`;
            }

            return `${payment.price} ${payment.currency}`;
        }
    };

    const getCryptoIconName = (name: string) => {
        // ETH or ETH_USDT => "eth" or "usdt"
        const lowered = name.toLowerCase();

        return lowered.includes("_") ? lowered.split("_")[1] : lowered;
    };

    const getCurCryptoIconName = () => {
        if (!paymentMethod || !payment) {
            return "error";
        }

        return getCryptoIconName(paymentMethod.name);
    };

    const genDropDownList = () => {
        const resList: DropDownItem[] = [];

        if (!payment?.paymentMethod) {
            resList.push({value: "empty value", key: "emptyValue", displayName: "Select crypto currency"});
        }

        availableMethods.map((availableMethod) => {
            resList.push({
                value: availableMethod.ticker,
                key: availableMethod.ticker,
                displayName: availableMethod.displayName
            });
        });

        return resList;
    };

    const getCurDropDownItem = () => {
        if (!payment?.paymentMethod) {
            return undefined;
        }

        return {
            value: payment.paymentMethod.ticker,
            key: payment.paymentMethod.ticker,
            displayName: payment.paymentMethod.displayName
        };
    };

    const submitButtonDisabled = Boolean(
        formikConfig.errors["email"] ||
            formikConfig.values.email === "" ||
            !paymentMethod ||
            !emailFilled ||
            !currencyChosen
    );

    const convertedResultRendered = renderConvertedResult(convertResult?.cryptoAmount, paymentMethod?.displayName);

    return (
        <div className="relative">
            {!payment && (
                <>
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon marginBottom={2} />
                    <LoadingTextIcon />
                </>
            )}

            {payment?.isLocked === true && payment.paymentInfo && payment.paymentMethod && paymentProcessError && (
                <>
                    <ProgressCircle
                        expiresAt={payment.paymentInfo.expiresAt}
                        minutesCount={payment.paymentInfo.expirationDurationMin}
                        error={paymentProcessError}
                    />
                    <h2 className="block mx-auto text-sm font-medium text-card-desc text-center">
                        Error occurred while processing your payment.
                    </h2>
                </>
            )}

            {payment?.isLocked === true && payment.paymentInfo && payment.paymentMethod && !paymentProcessError && (
                <>
                    <ProgressCircle
                        expiresAt={payment.paymentInfo.expiresAt}
                        minutesCount={payment.paymentInfo.expirationDurationMin}
                        error={paymentProcessError}
                    />
                    <span className="block font-medium text-center text-2xl mb-1">{getPrice()}</span>
                    <h2 className="block mx-auto text-sm font-medium text-card-desc text-center mb-5 sm:mb-4 sm:hidden">
                        Waiting for payment. Scan the QR code in your app or enter payment information manually
                    </h2>
                    <h2 className="block mx-auto text-sm font-medium text-card-desc text-center mb-5 sm:mb-4 lg:hidden">
                        Waiting for payment. Please send required crypto amount to specified address below.
                    </h2>
                    <div className="flex relative justify-center mb-7 sm:hidden">
                        <QRCodeSVG size={180} level={"H"} value={payment.paymentInfo.paymentLink} />
                        <Icon
                            name={getCryptoIconName(payment.paymentMethod.ticker)}
                            dir="crypto"
                            className="absolute p-1 w-12 h-12 bg-white border rounded-full left-1/2 -translate-y-1/2 top-1/2 -translate-x-1/2"
                        />
                    </div>
                    <span className="block mx-auto text-sm mb-7 font-medium text-center text-card-desc sm:hidden">
                        or
                    </span>

                    <div className="mx-auto h-16 w-16 flex items-center justify-center mb-3.5 lg:hidden">
                        <div className="shrink-0">
                            <Icon
                                name={getCryptoIconName(payment.paymentMethod.ticker)}
                                dir="crypto"
                                className="h-16 w-16"
                            />
                        </div>
                    </div>

                    <CopyButton
                        textToCopy={payment.paymentInfo.recipientAddress}
                        displayText={payment.paymentInfo.recipientAddress}
                    />
                    <CopyButton
                        textToCopy={payment.paymentInfo.amountFormatted}
                        displayText={payment.paymentInfo.amountFormatted + " " + payment.paymentMethod.displayName}
                    />
                </>
            )}

            {payment && !payment.isLocked && (
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
                    <span className="block mx-auto text-sm font-medium text-card-desc text-center max-w-sm-desc-size mb-8 sm:mb-3">
                        {payment?.description || <i>No description provided</i>}
                    </span>
                    <form onSubmit={formikConfig.handleSubmit}>
                        <div className="relative flex items-center mb-6">
                            {paymentMethod && (
                                <Icon name={getCurCryptoIconName()} dir="crypto" className="absolute h-6 w-6 left-4" />
                            )}

                            <DropDown
                                onChange={onSelectPaymentMethod}
                                items={genDropDownList()}
                                getIconName={getCryptoIconName}
                                iconsDir="crypto"
                                firstSelectedItem={getCurDropDownItem()}
                            />
                        </div>
                        <Input<EmailForm, "email">
                            id="email"
                            {...formikConfig}
                            handleBlur={(e: React.FocusEvent<string, Element>) =>
                                checkCustomer(e, formikConfig.values.email)
                            }
                            hasConvertedResult={convertResult !== undefined}
                            curValue={formikConfig.values.email}
                            error={!!formikConfig.errors["email"]}
                            value={formikConfig.values.email}
                        />
                        <span
                            className={`block font-medium text-center ${
                                convertResult ? "text-4xl text-[40px] mb-3" : "text-3xl mb-4"
                            }`}
                        >
                            {getPrice()}
                        </span>
                        {convertResult && paymentMethod && (
                            <span className="block font-medium text-center text-lg mb-4">
                                {convertedResultRendered ? convertedResultRendered : "loading.."}
                            </span>
                        )}
                        <div className="mx-auto flex items-center justify-center">
                            <button
                                className={`relative ${
                                    submitButtonDisabled ? "opacity-50" : ""
                                } border rounded-3xl bg-main-green-1 border-main-green-1 h-14 font-medium text-xl text-white flex items-center justify-center basis-full w-full`}
                                type="submit"
                                disabled={submitButtonDisabled}
                            >
                                Next
                                <Icon
                                    name="arrow_right_white"
                                    className="absolute h-5 w-5 right-24 xs:right-16 md:right-24"
                                />
                            </button>
                        </div>
                    </form>
                </>
            )}
        </div>
    );
};

export default PaymentPage;
