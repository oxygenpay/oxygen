import {useMutation, useQuery, useQueryClient, UseQueryResult} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import addressProvider from "src/providers/address-provider";
import {MerchantAddress, MerchantAddressParams} from "src/types";
import {sleep} from "src/utils";

const addressQueries = {
    listAddresses: (): UseQueryResult<MerchantAddress[]> => {
        const {merchantId} = useSharedMerchantId();

        return useQuery(
            ["listAddresses"],
            () => {
                return addressProvider.listAddresses(merchantId!);
            },
            {
                staleTime: Infinity,
                enabled: Boolean(merchantId),
                retry: 2
            }
        );
    },

    createAddress: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (params: MerchantAddressParams) => {
                return addressProvider.createAddress(merchantId!, params);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listAddresses"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    },

    updateAddress: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (params: MerchantAddress) => {
                return addressProvider.updateAddress(merchantId!, params.id, params.name);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listAddresses"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    },

    deleteAddress: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (addressId: string) => {
                return addressProvider.deleteAddress(merchantId!, addressId);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listAddresses"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    }
};

export default addressQueries;
