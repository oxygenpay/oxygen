import "./custom-card.scss";

import * as React from "react";
import bevis from "src/utils/bevis";
import {CheckCircleTwoTone} from "@ant-design/icons";

const b = bevis("custom-card");

interface Props {
    title: string;
    description: string;
    isActive: boolean;
    rightIcon?: React.ReactNode;
}

const CustomCard: React.FC<Props> = (props: Props) => {
    return (
        <div className={b()}>
            <div className={b("title-wrap")}>
                {props.isActive ? <CheckCircleTwoTone style={{marginRight: "10px"}} /> : null}
                <span className={b("title")}>{props.title}</span>
            </div>
            <div className={b("description")}>{props.description}</div>
            <div className={b("right-icon")}>{props.rightIcon}</div>
        </div>
    );
};

export default CustomCard;
