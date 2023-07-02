import * as React from "react";
import {flatten} from "lodash-es";

import {PageContainer} from "@ant-design/pro-components";
import {Button, Result, Table, Typography, Row, notification, FormInstance, Alert, Space, Dropdown} from "antd";
import {CheckOutlined, CopyOutlined, DeleteOutlined, MoreOutlined} from "@ant-design/icons";
import {ColumnsType} from "antd/es/table";
import {useMount} from "react-use";
import {CURRENCY_SYMBOL, PaymentLink, PaymentLinkParams} from "src/types";
import PaymentLinkForm from "src/components/payment-link-create/payment-link-create";
import paymentLinkQueriers from "src/queries/payment-link-queries";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import DrawerForm from "src/components/drawer-form/drawer-form";
import TimeLabel from "src/components/time-label/time-label";
import {sleep} from "src/utils";
import copyToClipboard from "src/utils/copy-to-clipboard";
import createConfirmPopup from "src/utils/create-confirm-popup";
import PaymentLinkDescCard from "src/components/payment-link-desc-card/payment-link-desc-card";

const PaymentLinksPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const listPaymentLinks = paymentLinkQueriers.listPaymentLinks();
    const createPaymentLink = paymentLinkQueriers.createPaymentLink();
    const deletePaymentLink = paymentLinkQueriers.deletePaymentLink();
    const [isFormOpen, setFormOpen] = React.useState<boolean>(false);
    const [openedCard, changeOpenedCard] = React.useState<PaymentLink[]>([]);
    const payments = flatten((listPaymentLinks.data || []).map((page) => page));
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const {merchantId} = useSharedMerchantId();

    const isLoading =
        listPaymentLinks.isLoading ||
        createPaymentLink.isLoading ||
        listPaymentLinks.isFetching ||
        deletePaymentLink.isLoading;

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const deleteSelectedLink = async (paymentLinkId: string) => {
        try {
            setIsFormSubmitting(true);
            await deletePaymentLink.mutateAsync(paymentLinkId);
            setFormOpen(false);
            openNotification("Payment Link was successfully deleted", "");
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const columns: ColumnsType<PaymentLink> = [
        {
            title: "Created At",
            dataIndex: "createdAt",
            key: "createdAt",
            render: (_, record) => <TimeLabel time={record.createdAt} />
        },
        {
            title: "URL",
            dataIndex: "url",
            key: "url",
            render: (_, record) => (
                <Space
                    style={{cursor: "pointer", marginLeft: "10px"}}
                    onClick={(e) => {
                        e.stopPropagation();
                        copyToClipboard(record.url, openNotification);
                    }}
                >
                    <span>{record.url}</span>
                    <CopyOutlined />
                </Space>
            )
        },
        {
            title: "Name",
            dataIndex: "name",
            key: "name",
            render: (_, record) => <span>{record.name}</span>
        },
        {
            title: "Price",
            dataIndex: "price",
            key: "price",
            width: "min-content",
            render: (_, record) => (
                <Row align="middle" justify="space-between">
                    {`${record.currency in CURRENCY_SYMBOL ? CURRENCY_SYMBOL[record.currency] : ""}${record.price}`}
                    <Space onClick={(e) => e.stopPropagation()}>
                        <Dropdown
                            menu={{
                                items: [
                                    {
                                        label: (
                                            <Space
                                                onClick={() =>
                                                    createConfirmPopup(
                                                        "Delete the link",
                                                        <span>Are you sure to delete this link?</span>,
                                                        () => deleteSelectedLink(record.id)
                                                    )
                                                }
                                            >
                                                <span>Delete</span>
                                                <Button type="text" danger icon={<DeleteOutlined />} />
                                            </Space>
                                        ),
                                        key: "0"
                                    }
                                ]
                            }}
                            trigger={["click"]}
                        >
                            <Button type="text" icon={<MoreOutlined style={{fontSize: "150%"}} />} />
                        </Dropdown>
                    </Space>
                </Row>
            )
        }
    ];

    useMount(async () => {
        if (merchantId) {
            return;
        }

        await sleep(1000);
        listPaymentLinks.remove();
        await listPaymentLinks.refetch();
    });

    React.useEffect(() => {
        listPaymentLinks.refetch();
    }, [merchantId]);

    const uploadCreatedPaymentLink = async (value: PaymentLinkParams, form: FormInstance<PaymentLinkParams>) => {
        try {
            setIsFormSubmitting(true);
            await createPaymentLink.mutateAsync(value);
            setFormOpen(false);
            openNotification("Payment Link was created", "");

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
                <Typography.Title>Payment Links</Typography.Title>
                <Button type="primary" onClick={() => setFormOpen(true)} style={{marginTop: 20}}>
                    New Payment Link
                </Button>
            </Row>
            <Alert
                style={{marginBottom: 20, width: "66%"}}
                message={
                    <>
                        <Typography.Text>
                            Links are the easy way of accepting payments from your customers. Each link
                        </Typography.Text>
                        <br />
                        <Typography.Text>represents a pre-defined payment with amount and description.</Typography.Text>
                    </>
                }
                type="info"
                showIcon
            />
            <Table
                columns={columns}
                dataSource={payments}
                rowKey={(record) => record.id}
                loading={isLoading}
                pagination={false}
                size="middle"
                locale={{
                    emptyText: (
                        <Result
                            icon={<></>}
                            title="Your payment links will be here"
                            subTitle="To create a payment link, click to the button at the right top of the table"
                        />
                    )
                }}
                onRow={(record) => ({
                    onClick: () => changeOpenedCard([record])
                })}
            />
            <DrawerForm
                title="Create a link"
                isFormOpen={isFormOpen}
                changeIsFormOpen={setFormOpen}
                formBody={
                    <PaymentLinkForm
                        onCancel={() => {
                            setFormOpen(false);
                        }}
                        onFinish={uploadCreatedPaymentLink}
                        isFormSubmitting={isFormSubmitting}
                    />
                }
            />
            <DrawerForm
                title="Link details"
                isFormOpen={Boolean(openedCard.length)}
                changeIsFormOpen={changeIsCardOpen}
                formBody={<PaymentLinkDescCard data={openedCard[0]} openNotificationFunc={openNotification} />}
                hasCloseBtn
                width={540}
            />
        </PageContainer>
    );
};

export default PaymentLinksPage;
