const renderStrippedStr = (url: string, leftLen = 36, rightLen = -10) => {
    return url.slice(0, leftLen) + "..." + url.slice(rightLen);
};

export default renderStrippedStr;
