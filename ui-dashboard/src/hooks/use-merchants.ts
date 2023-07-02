import React from "react";
import merchantProvider from "src/providers/merchant-provider";
import {useBetween} from "use-between";
import {Merchant} from "src/types";

interface MerchantsState {
    merchants: Merchant[] | undefined;
    getMerchants: () => Promise<Merchant[]>;
}

const useMerchants = (): MerchantsState => {
    const [merchants, setMerchants] = React.useState<Merchant[]>();

    const getMerchants = async () => {
        const merchants = await merchantProvider.listMerchants();
        setMerchants(merchants);
        return merchants;
    };

    return {
        merchants,
        getMerchants
    };
};

const useSharedMerchants = () => useBetween(useMerchants);

export default useSharedMerchants;
