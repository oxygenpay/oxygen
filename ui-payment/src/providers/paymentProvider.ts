import apiRequest from "src/utils/apiRequest";
import {CurrencyConvertResult, PaymentMethod, Payment, Customer, PaymentLink} from "src/types";

interface CurrencyConvertParams {
    fiatCurrency: string;
    fiatAmount: string;
    cryptoCurrency: string;
}

const PAYMENT_BASE_PATH = "/api/payment/v1";

const paymentProvider = {
    async currencyConvert(params: CurrencyConvertParams): Promise<CurrencyConvertResult> {
        const response = await apiRequest.get(PAYMENT_BASE_PATH + "/currency-convert", {params});
        return response.data;
    },

    async setPaymentMethod(paymentId: string, params: {ticker: string}): Promise<PaymentMethod> {
        const response = await apiRequest.post(PAYMENT_BASE_PATH + `/payment/${paymentId}/method`, params);
        return response.data;
    },

    async getSupportedMethods(paymentId: string): Promise<{availableMethods: PaymentMethod[]}> {
        const response = await apiRequest.get(PAYMENT_BASE_PATH + `/payment/${paymentId}/supported-method`);
        return response.data;
    },

    async getPayment(paymentId: string): Promise<Payment> {
        const response = await apiRequest.get(PAYMENT_BASE_PATH + `/payment/${paymentId}`);
        return response.data;
    },

    async getPaymentLink(paymentLinkId: string): Promise<PaymentLink> {
        const response = await apiRequest.get(PAYMENT_BASE_PATH + `/payment-link/${paymentLinkId}`);
        return response.data;
    },

    async createPaymentFromLink(paymentLinkId: string): Promise<string> {
        const response = await apiRequest.post(PAYMENT_BASE_PATH + `/payment-link/${paymentLinkId}/payment`);
        return response.data?.id;
    },

    async putPayment(paymentId: string): Promise<Payment> {
        const response = await apiRequest.put(PAYMENT_BASE_PATH + `/payment/${paymentId}`);
        return response.data;
    },

    async setCustomer(paymentId: string, params: {email: string}): Promise<Customer> {
        const response = await apiRequest.post(PAYMENT_BASE_PATH + `/payment/${paymentId}/customer`, params);
        return response.data;
    },

    async getCSRFCookie(): Promise<void> {
        const response = await apiRequest.get(PAYMENT_BASE_PATH + "/csrf-cookie");
        const csrf = response.headers["x-csrf-token"];

        apiRequest.defaults.headers.common["x-csrf-token"] = csrf;

        return response.data;
    }
};

export default paymentProvider;
