import "./balance-page.scss";

import * as React from "react";
import {flatten} from "lodash-es";
import {PageContainer} from "@ant-design/pro-components";
import {CheckOutlined, RightOutlined} from "@ant-design/icons";
import {Result, Space, Table, Tag, Typography, Row, Button, notification, FormInstance} from "antd";
import bevis from "src/utils/bevis";
import {ColumnsType} from "antd/es/table";
import {MerchantBalance, Payment, Withdrawal, MerchantAddress, CURRENCY_SYMBOL} from "src/types";
import CollapseString from "src/components/collapse-string/collapse-string";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import balancesQueries from "src/queries/balances-queries";
import addressQueries from "src/queries/address-queries";
import Icon from "src/components/icon/icon";
import WithdrawForm from "src/components/withdraw-form/withdraw-form";
import DrawerForm from "src/components/drawer-form/drawer-form";
import PaymentStatus from "src/components/payment-status/payment-status";
import WithdrawalDescCard from "src/components/withdraw-desc-card/withdraw-desc-card";
import TimeLabel from "src/components/time-label/time-label";
import {sleep} from "src/utils";

const b = bevis("balance-page");

const BalancePage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const listBalances = balancesQueries.listBalances();
    const listWithdrawals = balancesQueries.listWithdrawal();
    const createWithdrawal = balancesQueries.createWithdrawal();
    const listAddresses = addressQueries.listAddresses();
    const [balances, setBalances] = React.useState<MerchantBalance[]>(listBalances.data?.pages[0] || []);
    const [withdrawals, setWithdrawals] = React.useState<Payment[]>(
        flatten((listWithdrawals.data?.pages || []).map((page) => page.results))
    );
    const [addresses, setAddresses] = React.useState<MerchantAddress[]>(listAddresses.data || []);
    const [openedCard, changeOpenedCard] = React.useState<Payment[]>([]);
    const [activeWithdrawal, setActiveWithdrawal] = React.useState<MerchantBalance[]>([]);
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const {merchantId} = useSharedMerchantId();

    const renderIconName = (name: string) => {
        // ETH or ETH_USDT => "eth" or "usdt"
        const lowered = name.toLowerCase();

        return lowered.includes("_") ? lowered.split("_")[1] : lowered;
    };

    const balancesColumns: ColumnsType<MerchantBalance> = [
        {
            title: "Network",
            dataIndex: "network",
            key: "network",
            render: (_, record) => <span style={{whiteSpace: "nowrap"}}>{record.blockchainName}</span>
        },
        {
            title: "Currency",
            dataIndex: "currency",
            key: "currency",
            width: "min-content",
            render: (_, record) => (
                <Space align="center">
                    <Icon name={renderIconName(record.ticker.toLowerCase())} dir="crypto" className={b("icon")} />
                    <span style={{whiteSpace: "nowrap"}}> {record.currency} </span>
                </Space>
            )
        },
        {
            title: "Balance",
            dataIndex: "balance",
            key: "balance",
            render: (_, record) => (
                <Row align="middle" justify="space-between">
                    <Space>
                        <CollapseString
                            style={{marginRight: "10px"}}
                            text={`${record.amount} ${record.ticker}`}
                            collapseAt={12}
                            withPopover
                        />
                    </Space>
                </Row>
            )
        },
        {
            title: "USD Balance",
            dataIndex: "usdBalance",
            key: "usdBalance",
            render: (_, record) => (
                <Row align="middle" justify="space-between">
                    <Space>
                        <CollapseString
                            style={{marginRight: "10px"}}
                            text={`${CURRENCY_SYMBOL["USD"]}${record.usdAmount}`}
                            collapseAt={12}
                            withPopover
                        />
                        {record.isTest ? <Tag color="yellow">Test Balance</Tag> : null}
                    </Space>
                    <Button className={b("withdraw-btn")} onClick={() => setActiveWithdrawal([record])}>
                        Withdraw funds
                        <RightOutlined />
                    </Button>
                </Row>
            )
        }
    ];

    const emptyBalance: MerchantBalance = {
        amount: "empty",
        usdAmount: "empty",
        blockchain: "empty",
        blockchainName: "empty",
        currency: "empty",
        id: "empty",
        isTest: false,
        minimalWithdrawalAmountUSD: "empty",
        name: "empty",
        ticker: "ETH"
    };

    const withdrawalsColumns: ColumnsType<Payment> = [
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
            render: (_, record: Payment) => <PaymentStatus status={record.status} />
        },
        {
            title: "Network",
            dataIndex: "network",
            key: "network",
            width: "min-content",
            render: (_, record) => (
                <span style={{whiteSpace: "nowrap"}}>
                    {balances.find((item) => item.id === record.additionalInfo?.withdrawal?.balanceId)
                        ?.blockchainName ?? "loading..."}
                </span>
            )
        },
        {
            title: "Amount",
            dataIndex: "amount",
            key: "amount",
            render: (_, record) => (
                <span style={{whiteSpace: "nowrap"}}>
                    {`${record.price} ${
                        balances.find((item) => item.id === record.additionalInfo?.withdrawal?.balanceId)?.ticker ??
                        "loading..."
                    }`}
                </span>
            )
        },
        {
            title: "Address",
            dataIndex: "address",
            key: "address",
            render: (_, record) => (
                <span style={{whiteSpace: "nowrap"}}>
                    {addresses.find((item) => item.id === record.additionalInfo?.withdrawal?.addressId)?.address ??
                        record.additionalInfo?.withdrawal?.addressId}
                </span>
            )
        }
    ];

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const isLoadingBalance = listBalances.isLoading || listBalances.isFetching;
    const isLoadingWithdrawal = listWithdrawals.isLoading || listWithdrawals.isFetching;

    React.useEffect(() => {
        setBalances(listBalances.data?.pages[0] || []);
    }, [listBalances.data]);

    React.useEffect(() => {
        setWithdrawals(flatten((listWithdrawals.data?.pages || []).map((page) => page.results)));
    }, [listWithdrawals.data]);

    React.useEffect(() => {
        setAddresses(listAddresses.data || []);
    }, [listAddresses.data]);

    React.useEffect(() => {
        listBalances.refetch();
        listWithdrawals.refetch();
        listAddresses.refetch();
    }, [merchantId]);

    const withdrawFund = async (value: Withdrawal, form: FormInstance<Withdrawal>) => {
        try {
            setIsFormSubmitting(true);
            await createWithdrawal.mutateAsync(value);
            setActiveWithdrawal([]);
            openNotification(
                "Withdrawal created",
                `You have successfully created a new withdrawal with total amount of ${value.amount} ${
                    balances.find((item) => item.id === value.balanceId)?.ticker
                }`
            );

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const changeWithdrawalBalance = (value: boolean) => {
        if (!value) {
            setActiveWithdrawal([]);
        }
    };

    const changeWithdrawalActiveCard = (value: boolean) => {
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
            <Typography.Title>Balances</Typography.Title>
            <Table
                columns={balancesColumns}
                dataSource={balances}
                rowKey={(record) => record.id}
                className={b("row")}
                loading={isLoadingBalance}
                pagination={false}
                size="middle"
                locale={{
                    emptyText: (
                        <Result
                            icon={null}
                            title="Your balances will appear here after you receive any payment from a customer"
                        ></Result>
                    )
                }}
            />
            <Typography.Title>Withdrawals</Typography.Title>
            <Table
                columns={withdrawalsColumns}
                dataSource={withdrawals}
                rowKey={(record) => record.id}
                loading={isLoadingWithdrawal}
                pagination={false}
                size="middle"
                footer={() => (
                    <Button
                        type="primary"
                        onClick={() => listWithdrawals.fetchNextPage()}
                        disabled={!listWithdrawals.hasNextPage}
                    >
                        Load more
                    </Button>
                )}
                locale={{
                    emptyText: (
                        <Result
                            icon={null}
                            title="Your withdrawals will be here"
                            subTitle="To create a withdrawal, click to the Withdraw funds button at the balances table"
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
                title="Withdraw funds"
                isFormOpen={Boolean(activeWithdrawal.length)}
                changeIsFormOpen={changeWithdrawalBalance}
                formBody={
                    <WithdrawForm
                        onCancel={() => {
                            setActiveWithdrawal([]);
                        }}
                        onFinish={withdrawFund}
                        balance={activeWithdrawal[0] ? activeWithdrawal[0] : emptyBalance}
                        addresses={addresses}
                        openPopupFunc={openNotification}
                        isFormSubmitting={isFormSubmitting}
                    />
                }
                width={500}
            />
            <DrawerForm
                title="Withdrawal details"
                isFormOpen={Boolean(openedCard.length)}
                changeIsFormOpen={changeWithdrawalActiveCard}
                formBody={
                    <WithdrawalDescCard
                        data={openedCard[0]}
                        balances={balances}
                        addresses={addresses}
                        openNotificationFunc={openNotification}
                    />
                }
                width={530}
            />
        </PageContainer>
    );
};

export default BalancePage;
