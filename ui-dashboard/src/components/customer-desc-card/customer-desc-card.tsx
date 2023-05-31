import * as React from "react";
import {Descriptions, Button, Table} from "antd";
import {ColumnsType} from "antd/es/table";
import {RightOutlined} from "@ant-design/icons";
import bevis from "src/utils/bevis";
import {Customer, CURRENCY_SYMBOL, Payment, CustomerPayment} from "src/types";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import customersQueries from "src/queries/customer-queries";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import paymentsQueries from "src/queries/payments-queries";
import DrawerForm from "src/components/drawer-form/drawer-form";
import PaymentDescCard from "src/components/payment-desc-card/payment-desc-card";
import TimeLabel from "src/components/time-label/time-label";

interface Props {
    data?: Customer;
    openNotificationFunc: (title: string, description: string) => void;
}

const b = bevis("withdraw-desc-card");

const CustomerDescCard: React.FC<Props> = (props: Props) => {
    const getCustomer = customersQueries.getCustomer();
    const getPayment = paymentsQueries.getPayment();
    const [customer, setCustomer] = React.useState<Customer>();
    const [openedCard, changeOpenedCard] = React.useState<Payment[]>([]);

    const loadPayment = async (orderId: string) => {
        const response = await getPayment.mutateAsync(orderId);

        changeOpenedCard([response]);
    };

    const columns: ColumnsType<CustomerPayment> = [
        {
            title: "Created At",
            dataIndex: "createdAt",
            key: "createdAt",
            width: "25%",
            render: (_, record) => <TimeLabel time={record.createdAt} />
        },
        {
            title: "Status",
            dataIndex: "status",
            key: "status",
            render: (_, record) => <PaymentStatusLabel status={record.status} />
        },
        {
            title: "Price",
            dataIndex: "price",
            key: "price",
            width: "40%",
            render: (_, record) => <span>{`${CURRENCY_SYMBOL[record.currency]} ${record.price}`}</span>
        },
        {
            title: "Btn",
            dataIndex: "btn",
            key: "btn",
            render: (_, record) => (
                <Button type="primary" onClick={() => loadPayment(record.id)}>
                    View
                    <RightOutlined />
                </Button>
            )
        }
    ];

    const loadUser = async () => {
        if (!props.data || props.data.id === "empty") {
            return;
        }

        const response = await getCustomer.mutateAsync(props.data.id);
        setCustomer(response);
    };

    React.useEffect(() => {
        if (props.data) {
            loadUser();
        }
    }, [props.data]);

    const changeIsDescFormOpen = (value: boolean) => {
        if (!value) {
            changeOpenedCard([]);
        }
    };

    const isLoading = getCustomer.isLoading || getPayment.isLoading;

    return (
        <>
            <SpinWithMask isLoading={isLoading} />
            {!isLoading && customer?.details && (
                <>
                    <Descriptions>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>ID</span>}>
                            {customer.id}
                        </Descriptions.Item>
                        <Descriptions.Item span={3} label={<span className={b("item-title")}>Email</span>}>
                            {customer.email}
                        </Descriptions.Item>
                        <Descriptions.Item
                            span={3}
                            label={<span className={b("item-title")}>Successful Payments</span>}
                        >
                            {customer.details.successfulPayments}
                        </Descriptions.Item>

                        {Boolean(customer?.details?.payments.length) && (
                            <>
                                <Descriptions.Item
                                    span={3}
                                    label={<span className={b("item-title")}>Recent payments</span>}
                                >
                                    {null}
                                </Descriptions.Item>
                            </>
                        )}
                    </Descriptions>

                    {Boolean(customer?.details?.payments.length) && (
                        <>
                            <Table
                                columns={columns}
                                dataSource={customer?.details?.payments}
                                rowKey={(record) => record.id}
                                loading={isLoading}
                                pagination={false}
                                size="small"
                                showHeader={false}
                            />
                        </>
                    )}
                </>
            )}
            <DrawerForm
                title="Payment details"
                isFormOpen={Boolean(openedCard.length)}
                changeIsFormOpen={changeIsDescFormOpen}
                formBody={<PaymentDescCard data={openedCard[0]} openNotificationFunc={props.openNotificationFunc} />}
                hasCloseBtn
                width={530}
            />
        </>
    );
};

export default CustomerDescCard;
