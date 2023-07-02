import React from "react";
import {Payment} from "src/types/index";

interface PaymentContext {
    payment?: Payment;
    setPayment: (payment?: Payment) => void;
}

const Context = React.createContext<PaymentContext>({
    payment: undefined,
    setPayment: () => {}
});

function usePayment(): PaymentContext {
    return React.useContext(Context);
}

export default Context;
export {usePayment};
