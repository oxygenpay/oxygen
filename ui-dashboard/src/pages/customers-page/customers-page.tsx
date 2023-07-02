import * as React from "react";
import {flatten} from "lodash-es";
import {PageContainer} from "@ant-design/pro-components";
import {Button, Result, Table, Typography, notification} from "antd";
import {CheckOutlined} from "@ant-design/icons";
import {ColumnsType} from "antd/es/table";
import {Customer} from "src/types";
import CollapseString from "src/components/collapse-string/collapse-string";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import DrawerForm from "src/components/drawer-form/drawer-form";
import CustomerDescCard from "src/components/customer-desc-card/customer-desc-card";
import customersQueries from "src/queries/customer-queries";
import TimeLabel from "src/components/time-label/time-label";

const columns: ColumnsType<Customer> = [
    {
        title: "Created At",
        dataIndex: "createdAt",
        key: "createdAt",
        width: "25%",
        render: (_, record) => <TimeLabel time={record.createdAt} />
    },
    {
        title: "Email",
        dataIndex: "email",
        key: "email",
        render: (_, record) => <CollapseString text={record.email} collapseAt={32} withPopover />
    }
];

const CustomersPage: React.FC = () => {
    const [api, contextHolder] = notification.useNotification();
    const listCustomers = customersQueries.listCustomers();
    const [openedCard, changeOpenedCard] = React.useState<Customer[]>([]);
    const [customers, setCustomers] = React.useState<Customer[]>(
        flatten((listCustomers.data?.pages || []).map((page) => page.results))
    );
    const {merchantId} = useSharedMerchantId();

    const isLoading = listCustomers.isLoading || listCustomers.isFetching;

    React.useEffect(() => {
        setCustomers(flatten((listCustomers.data?.pages || []).map((page) => page.results)));
    }, [listCustomers.data]);

    React.useEffect(() => {
        listCustomers.refetch();
    }, [merchantId]);

    const changeIsCardOpen = (value: boolean) => {
        if (!value) {
            changeOpenedCard([]);
        }
    };

    const openNotification = (title: string, description: string) => {
        api.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    return (
        <PageContainer
            header={{
                title: "",
                breadcrumb: {}
            }}
        >
            {contextHolder}
            <Typography.Title>Customers</Typography.Title>
            <Table
                columns={columns}
                dataSource={customers}
                rowKey={(record) => record.id}
                loading={isLoading}
                pagination={false}
                size="large"
                footer={() => (
                    <Button
                        type="primary"
                        onClick={() => listCustomers.fetchNextPage()}
                        disabled={!listCustomers.hasNextPage}
                    >
                        Load more
                    </Button>
                )}
                locale={{
                    emptyText: <Result icon={<></>} title="No data provided"></Result>
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
                title="Customer details"
                isFormOpen={Boolean(openedCard.length)}
                changeIsFormOpen={changeIsCardOpen}
                formBody={<CustomerDescCard data={openedCard[0]} openNotificationFunc={openNotification} />}
                width={600}
            />
        </PageContainer>
    );
};

export default CustomersPage;
