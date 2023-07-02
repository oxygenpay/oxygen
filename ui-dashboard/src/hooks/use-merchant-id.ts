import React from "react";
import {useBetween} from "use-between";
import localStorage from "src/utils/local-storage";

interface MerchantIdState {
    merchantId: string | null;
    setMerchantId: (merchantId: string | null) => void;
}

const useMerchantId = (): MerchantIdState => {
    const [merchantId, setMerchantId] = React.useState<string | null>(localStorage.get("merchantId"));

    const customSetMerchantId = (merchantId: string | null) => {
        setMerchantId(merchantId);
        if (merchantId) {
            localStorage.set("merchantId", merchantId);
        }
    };

    return {
        merchantId,
        setMerchantId: customSetMerchantId
    };
};

const useSharedMerchantId = () => useBetween(useMerchantId);

export default useSharedMerchantId;
