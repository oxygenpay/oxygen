const copyToClipboard = (textToCopy: string, openNotificationFunc: (title: string, description: string) => void) => {
    navigator.clipboard.writeText(textToCopy).then(
        () => {
            openNotificationFunc("", "Text has been copied to your clipboard");
        },

        (err) => {
            console.error("failed to copy text", err.message);
        }
    );
};

export default copyToClipboard;
