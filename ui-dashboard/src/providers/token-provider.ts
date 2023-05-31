import apiRequest from "src/utils/api-request";
import {MerchantToken} from "src/types";
import withApiPath from "src/utils/with-api-path";

const tokenProvider = {
    async listTokens(merchantId: string): Promise<MerchantToken[]> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/token`));
        return response.data?.results;
    },

    async createToken(merchantId: string, name: string): Promise<MerchantToken> {
        const response = await apiRequest.post(withApiPath(`/merchant/${merchantId}/token`), {name});
        return response.data;
    },

    async deleteToken(merchantId: string, tokenId: string): Promise<MerchantToken> {
        const response = await apiRequest.delete(withApiPath(`/merchant/${merchantId}/token/${tokenId}`));
        return response.data;
    }
};

export default tokenProvider;
