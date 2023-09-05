interface MerchantBase {
    name: string;
    website: string;
}

interface Merchant extends MerchantBase {
    id: string;
    settings: string;
    webhookSettings: WebhookSettings;
    supportedPaymentMethods: PaymentMethod[];
}

interface WebhookSettings {
    secret?: string;
    url: string;
}

const BLOCKCHAIN = ["ETH", "TRON", "MATIC", "BSC"] as const;
type Blockchain = typeof BLOCKCHAIN[number];

const BLOCKCHAIN_TICKER = [
    "ETH",
    "ETH_USDT",
    "ETH_USDC",
    "MATIC",
    "MATIC_USDT",
    "MATIC_USDC",
    "TRON",
    "TRON_USDT",
    "BNB",
    "BSC_USDT",
    "BSC_BUSD"
] as const;

type BlockchainTicker = typeof BLOCKCHAIN_TICKER[number];

interface PaymentMethod {
    blockchain: Blockchain;
    blockchainName: string;
    displayName: string;
    enabled: boolean;
    name: string;
    ticker: BlockchainTicker;
}

interface MerchantToken {
    name: string;
    token: string;
    id: string;
    createdAt: string;
}

interface User {
    email: string;
    name: string;
    profileImageUrl: string;
    uuid: string;
}

interface AuthProvider {
    name: string;
}

interface UserCreateForm {
    email: string;
    password: string;
}

interface MerchantBalance {
    amount: string;
    usdAmount: string;
    blockchain: string;
    blockchainName: string;
    currency: string;
    id: string;
    isTest: boolean;
    minimalWithdrawalAmountUSD: string;
    name: string;
    ticker: BlockchainTicker;
}

interface Withdrawal {
    addressId: string;
    amount: string;
    balanceId: string;
}

const CURRENCY = ["USD", "EUR"] as const;
type Currency = typeof CURRENCY[number];
type CurrencyWithFiat = Currency | BlockchainTicker;

const CURRENCY_SYMBOL: Record<CurrencyWithFiat, string> = {
    USD: "$",
    EUR: "€",
    ETH: "",
    ETH_USDT: "",
    ETH_USDC: "",
    MATIC: "",
    MATIC_USDT: "",
    MATIC_USDC: "",
    TRON: "",
    TRON_USDT: "",
    BNB: "",
    BSC_USDT: "",
    BSC_BUSD: ""
};

type PaymentType = "payment" | "withdrawal";

type PaymentStatus = "pending" | "inProgress" | "success" | "failed";

interface ServiceFee {
    blockchain: string;
    calculatedAt: string;
    currency: string;
    currencyFee: string;
    isTest: boolean;
    usdFee: string;
}

interface PaymentParams {
    id: string;
    orderId?: number;
    price: number;
    currency: Currency;
    redirectUrl?: string;
    isTest?: boolean;
    description?: string;
}

interface AdditionalPaymentInfo {
    customerEmail: string;
    selectedCurrency: string;
    serviceFee: string;
}

interface AdditionalWithdrawalInfo {
    addressId: string;
    balanceId: string;
    explorerLink: string | null;
    serviceFee: string;
    transactionHash: string | null;
}

interface AdditionalInfo {
    payment?: AdditionalPaymentInfo;
    withdrawal?: AdditionalWithdrawalInfo;
}

interface Payment {
    additionalInfo?: AdditionalInfo;
    id: string;
    orderId?: string;
    type: PaymentType;
    status: PaymentStatus;
    createdAt: string;
    currency: CurrencyWithFiat;
    price: string;
    redirectUrl?: string;
    paymentUrl?: string;
    description?: string;
    isTest: boolean;
}

interface MerchantAddress {
    address: string;
    blockchain: string;
    blockchainName: string;
    id: string;
    name: string;
}

interface MerchantAddressParams {
    address: string;
    blockchain: string;
    name: string;
}

interface ListPaymentParams {
    limit?: number;
    cursor?: string;
    reverseOrder?: boolean;
    type: PaymentType;
}

type PaymentLinkAction = "redirect" | "showMessage";

interface PaymentLinkParams {
    currency: Currency;
    description?: string;
    name: string;
    price: number;
    redirectUrl?: string;
    successAction: PaymentLinkAction;
    successMessage?: string;
}

interface PaymentLink {
    createdAt: string;
    currency: Currency;
    description?: string;
    id: string;
    name: string;
    price: number;
    redirectUrl?: string;
    successAction: PaymentLinkAction;
    successMessage?: string;
    url: string;
}

interface PaymentsPagination {
    limit: number;
    cursor: string;
    results: Payment[];
}

interface ErrorResponseItem {
    field: string;
    message: string;
}

interface ErrorResponse {
    errors: ErrorResponseItem[];
    message: string;
    status: string;
}

interface SupportMessage {
    topic: string;
    message: string;
}

interface CustomerPayment {
    createdAt: string;
    currency: Currency;
    id: string;
    price: string;
    status: PaymentStatus;
}

interface CustomerDetails {
    payments: CustomerPayment[];
    successfulPayments: number;
}

interface Customer {
    createdAt: string;
    details?: CustomerDetails;
    email: string;
    id: string;
}

interface CustomerPagination {
    cursor: string;
    limit: number;
    results: Customer[];
}

interface ListCustomersParams {
    limit?: number;
    cursor?: string;
    reverseOrder?: boolean;
}

interface ConvertParams {
    from: CurrencyWithFiat;
    amount: string;
    to: CurrencyWithFiat;
}

interface ConvertResult {
    convertedAmount: string;
    exchangeRate: number;
    from: CurrencyWithFiat;
    to: CurrencyWithFiat;
}

export type {
    MerchantBase,
    Merchant,
    MerchantToken,
    User,
    PaymentMethod,
    BlockchainTicker,
    PaymentParams,
    Payment,
    ListPaymentParams,
    PaymentsPagination,
    Currency,
    ErrorResponse,
    MerchantBalance,
    Withdrawal,
    MerchantAddress,
    MerchantAddressParams,
    WebhookSettings,
    ServiceFee,
    SupportMessage,
    Customer,
    CustomerPagination,
    ListCustomersParams,
    PaymentStatus,
    ConvertParams,
    ConvertResult,
    CustomerPayment,
    PaymentLinkParams,
    PaymentLink,
    PaymentLinkAction,
    UserCreateForm,
    AuthProvider
};
export {BLOCKCHAIN, BLOCKCHAIN_TICKER, CURRENCY, CURRENCY_SYMBOL};
