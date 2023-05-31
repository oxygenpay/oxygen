import {useInfiniteQuery, UseInfiniteQueryResult, useQueryClient, useMutation} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import balancesProvider from "src/providers/balance-provider";
import merchantProvider from "src/providers/merchant-provider";
import {MerchantBalance, PaymentsPagination, Withdrawal, ConvertParams} from "src/types";
import {sleep} from "src/utils";

const PAGE_SIZE = 50;

const balancesQueries = {
    listBalances: (): UseInfiniteQueryResult<MerchantBalance[]> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery(
            ["listBalances"],
            () => {
                return balancesProvider.listBalances(merchantId!);
            },
            {
                staleTime: Infinity,
                enabled: Boolean(merchantId),
                retry: 2
            }
        );
    },

    listWithdrawal: (): UseInfiniteQueryResult<PaymentsPagination> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery(
            ["listWithdrawal"],
            ({pageParam = {cursor: "", type: "withdrawal"}}) => {
                return merchantProvider.listPayments(merchantId!, {
                    limit: PAGE_SIZE,
                    cursor: pageParam?.cursor || "",
                    type: pageParam.type,
                    reverseOrder: true
                });
            },
            {
                staleTime: Infinity,
                enabled: Boolean(merchantId),
                getNextPageParam: (lastPage) => {
                    if (!lastPage.cursor) {
                        return undefined;
                    }
                    return {
                        cursor: lastPage.cursor
                    };
                },
                retry: 2
            }
        );
    },

    createWithdrawal: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (params: Withdrawal) => {
                return balancesProvider.createWithdrawal(merchantId!, params);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listWithdrawal"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                    queryClient.invalidateQueries(["listBalances"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    },

    getServiceFee: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation((balanceId: string) => {
            return balancesProvider.getServiceFee(merchantId!, balanceId);
        });
    },

    getExchangeRate: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation((params: ConvertParams) => {
            return balancesProvider.getCurrencyExchangeRate(merchantId!, params);
        });
    }
};

export default balancesQueries;
