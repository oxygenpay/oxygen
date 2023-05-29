const renderConvertedResult = (amountFormatted: string | undefined, ticker: string | undefined) => {
    if (amountFormatted && ticker) {
        const sliceRes = amountFormatted.split(".");
        const amount = Number(amountFormatted);

        if (isNaN(amount)) {
            return null;
        }

        if (sliceRes[1]?.length < 8 || sliceRes.length < 2) {
            return amountFormatted + " " + ticker;
        }

        return amount.toFixed(7) + " " + ticker;
    }

    return null;
};

export default renderConvertedResult;
