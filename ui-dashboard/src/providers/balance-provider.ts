import apiRequest from "src/utils/api-request";
import {MerchantBalance, ServiceFee, Withdrawal, ConvertParams, ConvertResult} from "src/types";
import withApiPath from "src/utils/with-api-path";

const balancesProvider = {
    async listBalances(merchantId: string): Promise<MerchantBalance[]> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/balance`));
        return response.data?.results;
    },

    async getServiceFee(merchantId: string, balanceId: string): Promise<ServiceFee> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/withdrawal-fee`), {
            params: {balanceId}
        });
        return response.data;
    },

    async createWithdrawal(merchantId: string, params: Withdrawal): Promise<void> {
        await apiRequest.post(withApiPath(`/merchant/${merchantId}/withdrawal`), params);
        return;
    },

    async getCurrencyExchangeRate(merchantId: string, params: ConvertParams): Promise<ConvertResult> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/currency-convert`), {params});
        return response.data;
    }
};

export default balancesProvider;
