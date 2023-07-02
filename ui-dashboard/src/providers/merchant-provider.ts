import apiRequest from "src/utils/api-request";
import {
    MerchantBase,
    Merchant,
    BlockchainTicker,
    Payment,
    PaymentParams,
    ListPaymentParams,
    PaymentsPagination,
    WebhookSettings,
    SupportMessage,
    CustomerPagination,
    ListCustomersParams,
    Customer
} from "src/types";
import withApiPath from "src/utils/with-api-path";

const merchantProvider = {
    async storeMerchant(params: MerchantBase): Promise<Merchant> {
        const response = await apiRequest.post(withApiPath("/merchant"), params);
        return response.data;
    },

    async listMerchants(): Promise<Merchant[]> {
        const response = await apiRequest.get(withApiPath("/merchant"));
        return response.data?.results;
    },

    async showMerchant(merchantId: string): Promise<Merchant> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}`));
        return response.data;
    },

    async deleteMerchant(merchantId: string): Promise<Merchant> {
        const response = await apiRequest.delete(withApiPath(`/merchant/${merchantId}`));
        return response.data;
    },

    async updateMerchant(merchantId: string, params: MerchantBase): Promise<Merchant> {
        const response = await apiRequest.put(withApiPath(`/merchant/${merchantId}`), params);
        return response.data;
    },

    async updateMerchantWebhookSettings(merchantId: string, params: WebhookSettings): Promise<void> {
        await apiRequest.put(withApiPath(`/merchant/${merchantId}/webhook`), params);
        return;
    },

    async listPayments(merchantId: string, params?: ListPaymentParams): Promise<PaymentsPagination> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/payment`), {params});
        return response.data;
    },

    async getPayment(merchantId: string, paymentId: string): Promise<Payment> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/payment/${paymentId}`));
        return response.data;
    },

    async createPayment(merchantId: string, params: PaymentParams): Promise<Payment> {
        const response = await apiRequest.post(withApiPath(`/merchant/${merchantId}/payment`), params);
        return response.data;
    },

    async updateSupportedMethods(
        merchantId: string,
        params: {supportedPaymentMethods: BlockchainTicker[]}
    ): Promise<void> {
        const response = await apiRequest.put(withApiPath(`/merchant/${merchantId}/supported-method`), params);
        return response.data;
    },

    async sendSupportMessage(merchantId: string, params: SupportMessage): Promise<void> {
        await apiRequest.post(withApiPath(`/merchant/${merchantId}/form`), params);
        return;
    },

    async listCustomers(merchantId: string, params?: ListCustomersParams): Promise<CustomerPagination> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/customer`), {params});
        return response.data;
    },

    async getCustomerDetails(merchantId: string, customerId: string): Promise<Customer> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/customer/${customerId}`));
        return response.data;
    }
};

export default merchantProvider;
