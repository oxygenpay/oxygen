import "./custom-collapse.scss";

import * as React from "react";
import {useMount} from "react-use";
import bevis from "src/utils/bevis";

const b = bevis("custom-collapse");

interface Props {
    text: string;
    cn?: string;
}

const CustomCollapse: React.FC<Props> = (props: Props) => {
    const [isCollapsed, setIsCollapsed] = React.useState(true);
    const [canBeCollapsed, setCanBeCollapsed] = React.useState(true);
    const textRef = React.useRef<HTMLDivElement>(null);

    useMount(() => {
        if (!(textRef.current!.offsetWidth < textRef.current!.scrollWidth)) {
            setCanBeCollapsed(false);
            setIsCollapsed(false);
        }
    });

    const toggle = () => {
        setIsCollapsed(!isCollapsed);
    };

    return (
        <div className={b({collapsed: canBeCollapsed && isCollapsed})}>
            <div className={[b("text"), props.cn].filter(Boolean).join(" ")} ref={textRef}>
                {props.text}
            </div>
            {canBeCollapsed ? (
                <a className={b("button")} onClick={toggle}>
                    {isCollapsed ? "More" : "Hide"}
                </a>
            ) : null}
        </div>
    );
};

export default CustomCollapse;
