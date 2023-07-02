import {useMutation, useQuery, useQueryClient, UseQueryResult} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import tokenProvider from "src/providers/token-provider";
import {MerchantToken} from "src/types";
import {sleep} from "src/utils";

const tokenQueries = {
    listTokens: (): UseQueryResult<MerchantToken[]> => {
        const {merchantId} = useSharedMerchantId();

        return useQuery(
            ["listTokens"],
            () => {
                return tokenProvider.listTokens(merchantId!);
            },
            {
                staleTime: Infinity,
                enabled: Boolean(merchantId),
                retry: 2
            }
        );
    },

    createToken: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (name: string) => {
                return tokenProvider.createToken(merchantId!, name);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listTokens"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    },

    deleteToken: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (tokenId: string) => {
                return tokenProvider.deleteToken(merchantId!, tokenId);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listTokens"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    }
};

export default tokenQueries;
