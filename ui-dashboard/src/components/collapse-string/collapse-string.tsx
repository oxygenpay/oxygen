import * as React from "react";
import {Popover} from "antd";

interface Props {
    text: string;
    collapseAt: number;
    withPopover?: boolean;
    popupText?: string;
    style?: object;
}

const CollapseString: React.FC<Props> = (props: Props) => {
    return (
        <Popover
            style={props.style ? props.style : undefined}
            placement="bottom"
            content={props.popupText ? props.popupText : props.text}
        >
            <span>
                {props.text.length > props.collapseAt * 2
                    ? props.text.slice(0, props.collapseAt) +
                      "..." +
                      props.text.slice(props.text.length - props.collapseAt)
                    : props.text}
            </span>
        </Popover>
    );
};

export default CollapseString;
