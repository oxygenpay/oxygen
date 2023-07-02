interface CurrencyConvertResult {
    cryptoAmount: string;
    cryptoCurrency: string;
    displayName: string;
    exchangeRate: number;
    fiatAmount: number;
    fiatCurrency: string;
    network: string;
}

interface PaymentMethod {
    blockchain: string;
    blockchainName: string;
    displayName: string;
    name: string;
    ticker: string;
}

interface Customer {
    email: string;
    id: string;
}

const CURRENCY = ["USD", "EUR"] as const;
type Currency = typeof CURRENCY[number];

type PaymentStatus = "pending" | "inProgress" | "success" | "failed";
type PaymentAction = "redirect" | "showMessage";

interface PaymentInfo {
    amount: string;
    amountFormatted: string;
    recipientAddress: string;
    status: PaymentStatus;
    successUrl?: string;
    expiresAt: string;
    expirationDurationMin: number;
    successAction?: PaymentAction;
    successMessage?: string;
    paymentLink: string;
}

interface Payment {
    currency: Currency;
    customer?: Customer;
    description?: string;
    id: string;
    isLocked: boolean;
    merchantName: string;
    paymentInfo?: PaymentInfo;
    paymentMethod?: PaymentMethod;
    price: number;
}

interface PaymentLink {
    currency: Currency;
    description?: string;
    merchantName: string;
    price: number;
}

export type {CurrencyConvertResult, PaymentMethod, Payment, Customer, PaymentLink};
