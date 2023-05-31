import {useInfiniteQuery, UseInfiniteQueryResult, useMutation, useQueryClient} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import merchantProvider from "src/providers/merchant-provider";
import {PaymentParams, PaymentsPagination} from "src/types";
import {sleep} from "src/utils";

const PAGE_SIZE = 50;

const paymentsQueries = {
    listPayments: (): UseInfiniteQueryResult<PaymentsPagination> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery(
            ["listPayments"],
            ({pageParam = {cursor: "", type: "payment"}}) => {
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

    createPayment: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (params: PaymentParams) => {
                return merchantProvider.createPayment(merchantId!, params);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listPayments"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    },

    getPayment: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation((paymentId: string) => {
            return merchantProvider.getPayment(merchantId!, paymentId);
        });
    }
};

export default paymentsQueries;
