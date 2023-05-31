import apiRequest from "src/utils/api-request";
import {MerchantAddress, MerchantAddressParams} from "src/types";
import withApiPath from "src/utils/with-api-path";

const addressProvider = {
    async listAddresses(merchantId: string): Promise<MerchantAddress[]> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/address`));
        return response.data?.results;
    },

    async createAddress(merchantId: string, params: MerchantAddressParams): Promise<MerchantAddress> {
        const response = await apiRequest.post(withApiPath(`/merchant/${merchantId}/address`), params);
        return response.data;
    },

    async getAddress(merchantId: string, addressId: string): Promise<MerchantAddress> {
        const response = await apiRequest.get(withApiPath(`/merchant/${merchantId}/address/${addressId}`));
        return response.data;
    },

    async updateAddress(merchantId: string, addressId: string, name: string): Promise<void> {
        await apiRequest.put(withApiPath(`/merchant/${merchantId}/address/${addressId}`), {name});
    },

    async deleteAddress(merchantId: string, addressId: string): Promise<void> {
        await apiRequest.delete(withApiPath(`/merchant/${merchantId}/address/${addressId}`));
    }
};

export default addressProvider;
