import * as React from "react";
import Icon from "./Icon";

interface DropDownItem {
    value: string;
    key: string;
    displayName: string;
}
interface Props {
    onChange: (value: string) => void;
    items: DropDownItem[];
    getIconName: (name: string) => string;
    iconsDir?: string;
    firstSelectedItem?: DropDownItem;
}

const DropDown: React.FC<Props> = (props: Props) => {
    const [isFocused, setIsFocused] = React.useState<boolean>(false);
    const [selectedItem, setSelectedItem] = React.useState<DropDownItem>(props.firstSelectedItem || props.items[0]);

    React.useEffect(() => {
        if (selectedItem.key !== "emptyValue") {
            props.onChange(selectedItem.value);
        }

        setIsFocused(false);
    }, [selectedItem]);

    const checkOutsideClick = (event: React.FocusEvent<HTMLButtonElement>) => {
        if (event.currentTarget === event.target) {
            setIsFocused(false);
        }
    };

    return (
        <>
            <button
                type="button"
                id="currency_list"
                className={`relative h-12 flex props.items-center appearance-none bg-white bg-clip-padding bg-no-repeat border border-main-green-3 rounded-xl w-full py-3 px-4 ${
                    selectedItem.key !== "emptyValue" ? "pl-12" : ""
                } text-sm focus:outline-none focus:shadow-none`}
                onClick={() => setIsFocused(!isFocused)}
                onBlur={checkOutsideClick}
            >
                <span className="font-medium text-sm text-black">{selectedItem.displayName}</span>

                {selectedItem.key !== "emptyValue" && (
                    <Icon
                        name={props.getIconName(selectedItem.value)}
                        dir={props.iconsDir}
                        className="absolute h-6 w-6 left-4"
                    />
                )}

                {!isFocused && (
                    <Icon name="arrow_drop_down_down" className="absolute h-6 w-6 right-3 top-1/2 -translate-y-1/2" />
                )}
                {isFocused && (
                    <>
                        <Icon name="arrow_drop_down_up" className="absolute h-6 w-6 right-3 top-1/2 -translate-y-1/2" />
                        <ul className="absolute top-14 left-1/2 -translate-x-1/2 z-50 max-h-96 overflow-auto scrollbar-thumb-gray-900 bg-white bg-clip-padding bg-no-repeat border border-main-green-3 rounded-xl w-full py-3 px-4">
                            {props.items.map((child, idx) => (
                                <li
                                    className={`flex h-12 props.items-center relative ${
                                        props.items[idx].key !== "emptyValue" ? "pl-8" : ""
                                    } hover:text-main-green-2 ${
                                        props.items[idx].key === "emptyValue" && selectedItem.key !== "emptyValue"
                                            ? "hidden"
                                            : ""
                                    }`}
                                    key={child.key}
                                    onClick={() => setSelectedItem(child)}
                                >
                                    <span className="font-medium text-left">{child.displayName}</span>
                                    {props.items[idx].key !== "emptyValue" && (
                                        <Icon
                                            name={props.getIconName(child.value)}
                                            dir={props.iconsDir}
                                            className="absolute h-6 w-6 left-0"
                                        />
                                    )}
                                    {selectedItem.key === child.key && (
                                        <Icon name="ok" className="absolute h-6 w-6 right-0" />
                                    )}
                                </li>
                            ))}
                        </ul>
                    </>
                )}
            </button>
        </>
    );
};

export default DropDown;

export type {DropDownItem};
