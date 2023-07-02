import * as React from "react";
import {Portal} from "react-portal";
import ReactDOM from "react-dom/client";

interface Props {
    errorText?: string;
}

const ErrorPortalRoot = ReactDOM.createRoot(document.getElementById("root-portal") as HTMLElement);
let timeoutId = setTimeout(() => {}, 0);

const ErrorAlert: React.FC<Props> = ({errorText}) => {
    const errorNode = React.useRef(document.getElementById("root-portal"));
    const errorDescription = errorText
        ? errorText
        : "Please contact the support at " + import.meta.env.VITE_SUPPORT_EMAIL;

    return (
        <Portal node={errorNode.current}>
            <div className="relative w-full h-full transition">
                <div className="absolute right-10 bottom-10 flex items-center justify-center bg-main-red-2 w-44 h-24 border border-main-error rounded-xl px-2">
                    <div>
                        <span className="block font-medium text-sm text-center text-main-error mb-1">
                            Something went wrong
                        </span>
                        <span className="block font-medium text-xs text-center text-main-error">
                            {errorDescription}
                        </span>
                    </div>
                </div>
            </div>
        </Portal>
    );
};

const RenderErrorAlert = (errorText?: string) => {
    clearTimeout(timeoutId);
    const componentLifeTime = 10000;
    ErrorPortalRoot.render(<ErrorAlert errorText={errorText} />);
    timeoutId = setTimeout(() => ErrorPortalRoot.render(null), componentLifeTime);
};

export default ErrorAlert;
export {RenderErrorAlert};
