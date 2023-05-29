import * as React from "react";
import Icon from "./Icon";
interface Props<T extends Dictionary, K extends keyof T> {
    id: K;
    handleChange: (e: React.ChangeEvent<T[K]>) => void;
    handleBlur: (e: React.FocusEvent<T[K]>) => void;
    hasConvertedResult: boolean;
    curValue: string;
    error: boolean;
    value: string;
}

const Input = <T extends Dictionary, K extends keyof T>(props: Props<T, K>): JSX.Element => {
    const [error, changeError] = React.useState<boolean>();

    React.useEffect(() => {
        const timerId = setTimeout(() => changeError(props.error), 700);

        return () => clearTimeout(timerId);
    }, [props.value, props.error]);

    return (
        <div
            className={`relative flex items-center justify-center ${
                props.hasConvertedResult ? "mb-[4.25rem] sm:mb-32" : "mb-28 sm:mb-44"
            }`}
        >
            <input
                className={`h-12 border border-main-green-3 appearance-none border rounded-xl
                    w-full py-3 px-4 leading-tight font-medium
                    focus:outline-none focus:shadow-outline
                    ${error ? "border-main-error" : ""} ${!error && props.curValue ? "border-main-green-1" : ""}
                `}
                id="email"
                type="email"
                placeholder="Email"
                onChange={props.handleChange}
                onBlur={props.handleBlur}
                value={props.value}
                spellCheck={false}
            />
            {!error && props.curValue && <Icon name="ok" className="absolute h-6 w-6 right-3" />}

            {error ? (
                <>
                    <Icon name="red_cross" className="absolute h-6 w-6 right-3" />
                    <span className="absolute -bottom-5 lock font-medium text-main-error text-xs text-center">
                        Invalid E-mail provided
                    </span>
                </>
            ) : null}
        </div>
    );
};

export default Input;
