import "./spin-with-mask.scss";

import * as React from "react";
import bevis from "src/utils/bevis";
import {Spin} from "antd";

const b = bevis("spin-with-mask");

interface Props {
    isLoading: boolean;
}

const SpinWithMask: React.FC<Props> = (props: Props) => {
    return props.isLoading ? (
        <div className={b("mask")}>
            <Spin />
        </div>
    ) : null;
};

export default SpinWithMask;
