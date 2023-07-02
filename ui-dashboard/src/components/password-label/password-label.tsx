import "./password-label.scss";

import * as React from "react";
import {EyeTwoTone, EyeInvisibleOutlined} from "@ant-design/icons";
import copyToClipboard from "src/utils/copy-to-clipboard";
import useWindowSize from "src/hooks/use-window-size";

interface Props {
    text: string;
    width?: number;
    popupFunc: (title: string, desc: string) => void;
}

const PasswordLabel: React.FC<Props> = (props: Props) => {
    const windowSize = useWindowSize();
    const [visible, setVisible] = React.useState<boolean>(false);
    const securedString = React.useRef("*".repeat(props.text.length));

    const copyText = () => {
        if (visible) {
            copyToClipboard(props.text, props.popupFunc);
        }
    };

    const stripPassword = () => {
        if (!windowSize?.width) {
            return null;
        }

        const curWindowSize: number = windowSize.width;

        if (curWindowSize && curWindowSize <= 1300) {
            return props.text.slice(0, 15) + "..." + props.text.slice(-15);
        }

        return props.text;
    };

    const stripSecureText = () => {
        if (!windowSize?.width) {
            return null;
        }

        const curWindowSize: number = windowSize.width;

        if (curWindowSize && curWindowSize <= 1300) {
            return securedString.current.slice(0, 40);
        }

        return securedString.current;
    };

    return (
        <div className="password-label">
            {visible ? (
                <>
                    <span className="password-label__input" onClick={() => copyText()}>
                        {stripPassword()}
                    </span>
                    <EyeTwoTone onClick={() => setVisible(false)} />
                </>
            ) : (
                <>
                    <span className="password-label__input" onClick={() => copyText()}>
                        {stripSecureText()}
                    </span>
                    <EyeInvisibleOutlined onClick={() => setVisible(true)} />
                </>
            )}
        </div>
    );
};

export default PasswordLabel;
