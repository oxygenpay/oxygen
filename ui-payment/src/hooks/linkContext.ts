import React from "react";
import {PaymentLink} from "src/types/index";

interface PaymentLinkContext {
    paymentLink?: PaymentLink;
    setPaymentLink: (paymentLink?: PaymentLink) => void;
}

const Context = React.createContext<PaymentLinkContext>({
    paymentLink: undefined,
    setPaymentLink: () => {}
});

function usePaymentLink(): PaymentLinkContext {
    return React.useContext(Context);
}

export default Context;
export {usePaymentLink};
