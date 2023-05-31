import * as React from "react";
import {Result, Table, Row, Typography, Space, Button, Dropdown, FormInstance} from "antd";
import {ColumnsType} from "antd/es/table";
import {MoreOutlined, DeleteOutlined, EditOutlined} from "@ant-design/icons";
import {MerchantAddress, MerchantAddressParams} from "src/types";
import addressQueries from "src/queries/address-queries";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import DrawerForm from "src/components/drawer-form/drawer-form";
import AddressCreateForm from "src/components/address-create-form/address-create-form";
import AddressEditForm from "src/components/address-edit-form/address-edit-form";
import createConfirmPopup from "src/utils/create-confirm-popup";
import {sleep} from "src/utils";

interface Props {
    openPopupFunc: (title: string, desc: string) => void;
}

interface EditAddressFormFields {
    name: string;
}

const WithdrawAddresses: React.FC<Props> = (props: Props) => {
    const listAddresses = addressQueries.listAddresses();
    const createAddress = addressQueries.createAddress();
    const updateAddress = addressQueries.updateAddress();
    const deleteAddress = addressQueries.deleteAddress();
    const [addresses, setAddresses] = React.useState<MerchantAddress[]>(listAddresses.data || []);
    const [isAddAddressFormOpen, setIsAddAddressFormOpen] = React.useState<boolean>(false);
    const [openedAddress, changeOpenedAddress] = React.useState<MerchantAddress[]>([]);
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);
    const {merchantId} = useSharedMerchantId();

    const isLoading =
        listAddresses.isLoading ||
        listAddresses.isFetching ||
        createAddress.isLoading ||
        updateAddress.isLoading ||
        deleteAddress.isLoading;

    const deleteSelectedAddress = async (value: MerchantAddress) => {
        try {
            await deleteAddress.mutateAsync(value.id);
            changeOpenedAddress([]);
            props.openPopupFunc("Address has deleted", `Address ${value.name} has been deleted`);
        } catch (error) {
            console.error("error occurred: ", error);
        }
    };

    const columns: ColumnsType<MerchantAddress> = [
        {
            title: "Network",
            dataIndex: "network",
            key: "network",
            width: "min-content",
            render: (_, record) => <span style={{whiteSpace: "nowrap"}}>{record.blockchainName}</span>
        },
        {
            title: "Name",
            dataIndex: "addressName",
            key: "addressName",
            width: "min-content",
            render: (_, record) => <span style={{whiteSpace: "nowrap"}}>{record.name}</span>
        },
        {
            title: "Address",
            dataIndex: "address",
            key: "address",
            render: (_, record) => (
                <Row align="middle" justify="space-between">
                    <span>{record.address}</span>
                    <Dropdown
                        menu={{
                            items: [
                                {
                                    label: (
                                        <Row
                                            align="middle"
                                            justify="space-between"
                                            onClick={() => changeOpenedAddress([record])}
                                        >
                                            <span>Edit</span>
                                            <Button type="text" icon={<EditOutlined />} />
                                        </Row>
                                    ),
                                    key: "0"
                                },
                                {
                                    label: (
                                        <Space
                                            onClick={() =>
                                                createConfirmPopup(
                                                    "Delete the address",
                                                    <span>Are you sure to delete this address?</span>,
                                                    () => deleteSelectedAddress(record)
                                                )
                                            }
                                        >
                                            <span>Delete</span>
                                            <Button type="text" danger icon={<DeleteOutlined />} />
                                        </Space>
                                    ),
                                    key: "1"
                                }
                            ]
                        }}
                        trigger={["click"]}
                    >
                        <Button type="text" icon={<MoreOutlined style={{fontSize: "150%"}} />} />
                    </Dropdown>
                </Row>
            )
        }
    ];

    React.useEffect(() => {
        setAddresses(listAddresses.data || []);
    }, [listAddresses.data]);

    React.useEffect(() => {
        listAddresses.refetch();
    }, [merchantId]);

    const uploadCreatedAddress = async (value: MerchantAddressParams, form: FormInstance<MerchantAddressParams>) => {
        try {
            setIsFormSubmitting(true);
            await createAddress.mutateAsync(value);
            setIsAddAddressFormOpen(false);
            props.openPopupFunc("Address has created", `You have created new address ${value.name}`);

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const uploadNewAddress = async (value: MerchantAddress, form: FormInstance<EditAddressFormFields>) => {
        try {
            setIsFormSubmitting(true);
            await updateAddress.mutateAsync(value);
            changeOpenedAddress([]);
            props.openPopupFunc("Address has updated", `You have updated your address to ${value.name}`);

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const setOpenedAddress = (value: boolean) => {
        if (!value) {
            changeOpenedAddress([]);
        }
    };

    return (
        <>
            <Row align="middle" justify="space-between">
                <Typography.Title level={3}>Withdrawal Addresses</Typography.Title>
                <Button type="primary" onClick={() => setIsAddAddressFormOpen(true)} style={{marginTop: 20}}>
                    Add address
                </Button>
            </Row>
            <Table
                columns={columns}
                dataSource={addresses}
                rowKey={(record) => record.id}
                loading={isLoading}
                pagination={false}
                size="middle"
                locale={{
                    emptyText: (
                        <Result
                            icon={<></>}
                            title="Your addresses will be here"
                            subTitle="To create an address, click to the button at the right top of the table"
                        ></Result>
                    )
                }}
            />
            <DrawerForm
                title="Create an address"
                isFormOpen={isAddAddressFormOpen}
                changeIsFormOpen={setIsAddAddressFormOpen}
                formBody={
                    <AddressCreateForm
                        onCancel={() => {
                            setIsAddAddressFormOpen(false);
                        }}
                        onFinish={uploadCreatedAddress}
                        isFormSubmitting={isFormSubmitting}
                    />
                }
            />
            <DrawerForm
                title="Edit withdrawal address"
                isFormOpen={Boolean(openedAddress.length)}
                changeIsFormOpen={setOpenedAddress}
                formBody={
                    <AddressEditForm
                        onCancel={() => {
                            changeOpenedAddress([]);
                        }}
                        onFinish={uploadNewAddress}
                        selectedAddress={openedAddress[0]}
                        isFormSubmitting={isFormSubmitting}
                    />
                }
            />
        </>
    );
};

export default WithdrawAddresses;
