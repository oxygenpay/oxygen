import {useMutation, useQuery, useQueryClient, UseQueryResult} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import paymentLinkProvider from "src/providers/payment-link-provider";
import {PaymentLink, PaymentLinkParams} from "src/types";
import {sleep} from "src/utils";

const paymentLinkQueriers = {
    listPaymentLinks: (): UseQueryResult<PaymentLink[]> => {
        const {merchantId} = useSharedMerchantId();

        return useQuery(
            ["listPaymentLinks"],
            () => {
                return paymentLinkProvider.listPaymentLinks(merchantId!);
            },
            {
                staleTime: Infinity,
                enabled: Boolean(merchantId),
                retry: 2
            }
        );
    },

    createPaymentLink: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (params: PaymentLinkParams) => {
                return paymentLinkProvider.createPaymentLink(merchantId!, params);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listPaymentLinks"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    },

    deletePaymentLink: () => {
        const {merchantId} = useSharedMerchantId();
        const queryClient = useQueryClient();

        return useMutation(
            (paymentLinkId: string) => {
                return paymentLinkProvider.deletePaymentLink(merchantId!, paymentLinkId);
            },
            {
                onSuccess: async () => {
                    await sleep(1000);
                    queryClient.invalidateQueries(["listPaymentLinks"], {
                        refetchPage: (_page, index, allPages) => index === allPages.length - 1
                    });
                }
            }
        );
    }
};

export default paymentLinkQueriers;
