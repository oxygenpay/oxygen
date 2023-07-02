import * as React from "react";
import Icon from "./Icon";
import useWindowSize from "src/hooks/useWindowSize";

interface Props {
    textToCopy: string;
    displayText: string;
}

const CopyButton: React.FC<Props> = ({textToCopy, displayText}) => {
    const windowSize = useWindowSize();
    const [copied, setCopied] = React.useState<boolean>(false);

    const copyToClipboard = () => {
        navigator.clipboard.writeText(textToCopy).then(
            () => {
                setCopied(true);

                setTimeout(() => {
                    setCopied(false);
                }, 1000);
            },

            (err) => {
                console.log("failed to copy text", err.message);
            }
        );
    };

    const stripAddress = () => {
        if (!windowSize?.width) {
            return null;
        }

        const curWindowSize: number = windowSize.width;

        if (curWindowSize && curWindowSize <= 370) {
            return displayText.slice(0, 10) + "..." + displayText.slice(-10);
        }

        if (displayText.length > 30) {
            return displayText.slice(0, 15) + "..." + displayText.slice(-15);
        }

        return displayText;
    };

    const btnCopiedStyle = copied ? "bg-main-green-3 text-center" : "";
    const strippedAddress = stripAddress();

    return (
        <div className="relative w-full mb-6">
            <button
                onClick={copyToClipboard}
                className={
                    btnCopiedStyle +
                    " h-12 py-3 px-4 text-left text-sm border transition-colors rounded-xl transition w-full border-main-green-3"
                }
                disabled={!strippedAddress}
            >
                <Icon name="copy_smth" className="absolute right-2" />
                {copied ? "Copied" : stripAddress()}
            </button>
        </div>
    );
};

export default CopyButton;
