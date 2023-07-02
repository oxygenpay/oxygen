import React from "react";
import merchantProvider from "src/providers/merchant-provider";
import {useBetween} from "use-between";
import {Merchant} from "src/types";

interface MerchantState {
    merchant: Merchant | undefined;
    getMerchant: (merchantId: string) => Promise<Merchant | undefined>;
}

const useMerchant = (): MerchantState => {
    const [merchant, setMerchant] = React.useState<Merchant>();

    const getMerchant = async (merchantId: string) => {
        const merchant = await merchantProvider.showMerchant(merchantId);
        setMerchant(merchant);
        return merchant;
    };

    return {
        merchant,
        getMerchant
    };
};

const useSharedMerchant = () => useBetween(useMerchant);

export default useSharedMerchant;
