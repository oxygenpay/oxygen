const renderCurrency = (currency?: string, price?: number) => {
    if (currency === undefined || price === undefined) {
        return;
    }

    if (currency === "USD") {
        return `$${price.toFixed(2)}`;
    }
    if (currency === "EUR") {
        return `€${price.toFixed(2)}`;
    }

    return `${price} ${currency}`;
};

export default renderCurrency;
