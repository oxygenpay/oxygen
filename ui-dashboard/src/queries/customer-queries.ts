import {useInfiniteQuery, UseInfiniteQueryResult, useMutation} from "@tanstack/react-query";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import merchantProvider from "src/providers/merchant-provider";
import {CustomerPagination} from "src/types";

const PAGE_SIZE = 50;

const customersQueries = {
    listCustomers: (): UseInfiniteQueryResult<CustomerPagination> => {
        const {merchantId} = useSharedMerchantId();

        return useInfiniteQuery(
            ["listCustomers"],
            ({pageParam = {cursor: ""}}) => {
                return merchantProvider.listCustomers(merchantId!, {
                    limit: PAGE_SIZE,
                    cursor: pageParam?.cursor || "",
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

    getCustomer: () => {
        const {merchantId} = useSharedMerchantId();

        return useMutation((customerId: string) => {
            return merchantProvider.getCustomerDetails(merchantId!, customerId);
        });
    }
};

export default customersQueries;
