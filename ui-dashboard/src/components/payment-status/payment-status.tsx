import {PaymentStatus} from "src/types";
import {Tag} from "antd";

interface Props {
    status: PaymentStatus;
}

const PaymentStatusLabel: React.FC<Props> = ({status}) => (
    <>
        {(() => {
            switch (status) {
                case "pending":
                    return <Tag color="blue">Pending</Tag>;
                case "inProgress":
                    return <Tag color="orange">In Progress</Tag>;
                case "success":
                    return <Tag color="green">Success</Tag>;
                default:
                    return <Tag color="red">Failed</Tag>;
            }
        })()}
    </>
);

export default PaymentStatusLabel;
