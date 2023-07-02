import {ReactElement} from "react";
import {Modal} from "antd";
import {ExclamationCircleFilled} from "@ant-design/icons";

const createConfirmPopup = (title: string, content: ReactElement, submitFunc: () => void) => {
    Modal.confirm({
        title,
        icon: <ExclamationCircleFilled />,
        content,
        okText: "Yes",
        okType: "danger",
        cancelText: "No",
        maskClosable: true,
        onOk() {
            submitFunc();
        },
        onCancel() {
            //...
        }
    });
};

export default createConfirmPopup;
