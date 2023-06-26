import "./payments-page.scss";

import * as React from "react";
import {flatten} from "lodash-es";

import {PageContainer} from "@ant-design/pro-components";
import {Button, Result, Table, Typography, Row, notification, FormInstance} from "antd";
import {CheckOutlined} from "@ant-design/icons";
import {ColumnsType} from "antd/es/table";
import {useMount} from "react-use";
import {CURRENCY_SYMBOL, Payment, PaymentParams} from "src/types";
import PaymentForm from "src/components/payment-add-form/payment-add-form";
import CollapseString from "src/components/collapse-string/collapse-string";
import paymentsQueries from "src/queries/payments-queries";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import DrawerForm from "src/components/drawer-form/drawer-form";
import PaymentDescCard from "src/components/payment-desc-card/payment-desc-card";
import PaymentStatusLabel from "src/components/payment-status/payment-status";
import TimeLabel from "src/components/time-label/time-label";
import {sleep} from "src/utils";

const columns: ColumnsType<Payment> = [
    {
        title: "Created At",
        dataIndex: "createdAt",
        key: "createdAt",
        render: (_, record) => <TimeLabel time={record.createdAt} />
    },
    {
        title: "Status",
        dataIndex: "curStatus",
        key: "curStatus",
        render: (_, record: Payment) => <PaymentStatusLabel status={record.status} />
    },
    {
        title: "Price",
        dataIndex: "price",
        key: "price",
        width: "min-content",
        render: (_, record) => (
            <span style={{whiteSpace: "nowrap"}}>
                {`${record.currency in CURRENCY_SYMBOL ? CURRENCY_SYMBOL[record.currency] : ""}${record.price}`}
            </span>
        )
    },
    {
        title: "Order ID",
        dataIndex: "orderId",
        key: "orderId",
        render: (_, record) => (
            <CollapseString text={!record.orderId ? "Not provided" : record.orderId} collapseAt={12} withPopover />
        )
    },
    {
        title: "Description",
        dataIndex: "description",
        key: "description",
        render: (_, record) =>
            record.description ? <CollapseString text={record.description} collapseAt={32} withPopover /> : null
    }
];

const PaymentsPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const listPayments = paymentsQueries.listPayments();
    const createPayment = paymentsQueries.createPayment();
    const [isFormOpen, setFormOpen] = React.useState<boolean>(false);
    const [openedCard, changeOpenedCard] = React.useState<Payment[]>([]);
    const [payments, setPayments] = React.useState<Payment[]>(
        flatten((listPayments.data?.pages || []).map((page) => page.results))
    );
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const {merchantId} = useSharedMerchantId();

    const isLoading = listPayments.isLoading || createPayment.isLoading || listPayments.isFetching;

    useMount(async () => {
        if (merchantId) {
            return;
        }

        await sleep(1000);
        listPayments.remove();

        await listPayments.refetch();
    });

    React.useEffect(() => {
        setPayments(flatten((listPayments.data?.pages || []).map((page) => page.results)));
    }, [listPayments.data]);

    React.useEffect(() => {
        listPayments.refetch();
    }, [merchantId]);

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const uploadCreatedPayment = async (value: PaymentParams, form: FormInstance<PaymentParams>) => {
        try {
            setIsFormSubmitting(true);
            await createPayment.mutateAsync(value);
            setFormOpen(false);
            openNotification("Payment was created", "");

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const changeIsCardOpen = (value: boolean) => {
        if (!value) {
            changeOpenedCard([]);
        }
    };

    return (
        <PageContainer
            header={{
                title: "",
                breadcrumb: {}
            }}
        >
            {contextHolder}
            <Row align="middle" justify="space-between">
                <Typography.Title>Payments</Typography.Title>
                <Button type="primary" onClick={() => setFormOpen(true)} style={{marginTop: 20}}>
                    New Payment
                </Button>
            </Row>
            <Table
                columns={columns}
                dataSource={payments}
                rowKey={(record) => record.id}
                rowClassName="payments-page__row"
                loading={isLoading}
                pagination={false}
                size="middle"
                footer={() => (
                    <Button
                        type="primary"
                        onClick={() => listPayments.fetchNextPage()}
                        disabled={!listPayments.hasNextPage}
                    >
                        Load more
                    </Button>
                )}
                locale={{
                    emptyText: (
                        <Result
                            icon={<></>}
                            title="Your orders will be here"
                            subTitle="To create an order, click to the button at the right top of the table"
                        ></Result>
                    )
                }}
                onRow={(record) => {
                    return {
                        onClick: () => {
                            changeOpenedCard([record]);
                        }
                    };
                }}
            />
            <DrawerForm
                title="Create payment"
                isFormOpen={isFormOpen}
                changeIsFormOpen={setFormOpen}
                formBody={
                    <PaymentForm
                        onCancel={() => {
                            setFormOpen(false);
                        }}
                        onFinish={uploadCreatedPayment}
                        isFormSubmitting={isFormSubmitting}
                    />
                }
            />
            <DrawerForm
                title="Payment details"
                isFormOpen={Boolean(openedCard.length)}
                changeIsFormOpen={changeIsCardOpen}
                formBody={<PaymentDescCard data={openedCard[0]} openNotificationFunc={openNotification} />}
                hasCloseBtn
                width={540}
            />
        </PageContainer>
    );
};

export default PaymentsPage;
