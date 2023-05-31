import apiRequest from "src/utils/api-request";
import {PaymentLinkParams, PaymentLink} from "src/types";
import withApiPath from "src/utils/with-api-path";

const paymentLinkProvider = {
    async listPaymentLinks(merchantId: string): Promise<PaymentLink[]> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/payment-link`));
        return response.data?.results;
    },

    async createPaymentLink(merchantId: string, params: PaymentLinkParams): Promise<PaymentLink> {
        const response = await apiRequest.post(withApiPath(`/merchant/${merchantId}/payment-link`), params);
        return response.data;
    },

    async getPaymentLink(merchantId: string, paymentLinkId: string): Promise<PaymentLink> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/payment-link/${paymentLinkId}`));
        return response.data;
    },

    async deletePaymentLink(merchantId: string, paymentLinkId: string): Promise<void> {
        await apiRequest.delete(withApiPath(`/merchant/${merchantId}/payment-link/${paymentLinkId}`));
    }
};

export default paymentLinkProvider;
