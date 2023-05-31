import * as React from "react";
import {useAsyncFn} from "react-use";
import {PageContainer} from "@ant-design/pro-components";
import {useNavigate} from "react-router-dom";
import {Button, Space, notification, Typography, FormInstance} from "antd";
import {DeleteOutlined, EditOutlined, CheckOutlined} from "@ant-design/icons";
import CustomCard from "src/components/custom-card/custom-card";
import useSharedMerchants from "src/hooks/use-merchants";
import merchantProvider from "src/providers/merchant-provider";
import {MerchantBase, Merchant} from "src/types";
import MerchantForm from "src/components/merchant-form/merchant-form";
import SpinWithMask from "src/components/spin-with-mask/spin-with-mask";
import {sleep} from "src/utils";
import useSharedMerchantId from "src/hooks/use-merchant-id";
import DrawerForm from "src/components/drawer-form/drawer-form";
import createConfirmPopup from "src/utils/create-confirm-popup";

const ManageMerchantsPage: React.FC = () => {
    const navigate = useNavigate();
    const [notificationApi, contextHolder] = notification.useNotification();
    const {merchants, getMerchants} = useSharedMerchants();
    const {merchantId, setMerchantId} = useSharedMerchantId();
    const [activeMerchant, setActiveMerchant] = React.useState<Merchant | undefined>();
    const [isAddMerchantFormOpened, setIsAddMerchantFormOpened] = React.useState<boolean>(false);
    const [isEditAddMerchantFormOpened, setIsEditAddMerchantFormOpened] = React.useState<boolean>(false);
    const [createMerchantReqState, createMerchantReq] = useAsyncFn(merchantProvider.storeMerchant);
    const [editMerchantReqState, editMerchantReq] = useAsyncFn(merchantProvider.updateMerchant);
    const [isFormSubmitting, setIsFormSubmitting] = React.useState<boolean>(false);

    const resetValues = async () => {
        await sleep(1000);
        setActiveMerchant(undefined);
        setIsAddMerchantFormOpened(false);
        setIsEditAddMerchantFormOpened(false);
        await getMerchants();
    };
    const [resetValuesFnState, resetValuesFn] = useAsyncFn(resetValues);

    const openNotification = (title: string, description: string) => {
        notificationApi.info({
            message: title,
            description,
            placement: "bottomRight",
            icon: <CheckOutlined style={{color: "#49D1AC"}} />
        });
    };

    const uploadCreatedMerchant = async (value: MerchantBase, form: FormInstance<MerchantBase>) => {
        try {
            setIsFormSubmitting(true);
            const merchant = await createMerchantReq(value);
            resetValuesFn();
            setMerchantId(merchant.id);
            openNotification("Welcome to the O2Pay", `Hello, ${value.name}!`);

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const uploadEditedMerchant = async (merchantId: string, value: MerchantBase, form: FormInstance<MerchantBase>) => {
        try {
            setIsFormSubmitting(true);
            await editMerchantReq(merchantId, value);
            resetValuesFn();
            openNotification("Information saved", `You have changed information about ${value.name} merchant`);

            await sleep(1000);
            form.resetFields();
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const onConfirmDeleteMerchant = async (merchant: Merchant) => {
        try {
            setIsFormSubmitting(true);
            await merchantProvider.deleteMerchant(merchant.id);
            await getMerchants();

            const nextMerchant = merchants?.find((merchantItem) => merchantItem.id !== merchant.id);

            if (nextMerchant) {
                setMerchantId(nextMerchant.id);
            }

            openNotification(`Merchant ${merchant.name} has been deleted`, "Thank you for being with us");
        } catch (error) {
            console.error("error occurred: ", error);
        } finally {
            setIsFormSubmitting(false);
        }
    };

    const onEdit = async (index: number) => {
        setIsEditAddMerchantFormOpened(true);
        setActiveMerchant(merchants![index]);
    };

    const isAnyFormOpen = isAddMerchantFormOpened || isEditAddMerchantFormOpened;
    const isTableLoading =
        createMerchantReqState.loading || editMerchantReqState.loading || resetValuesFnState.loading || !merchants;

    return (
        <PageContainer
            header={{
                title: (
                    <>
                        <Typography.Title>Manage Merchants</Typography.Title>
                        {merchants?.length === 0 && (
                            <Typography.Title level={3}>
                                To start accepting payments, first create a new merchant
                            </Typography.Title>
                        )}
                    </>
                ),
                breadcrumb: {}
            }}
        >
            {contextHolder}

            <div style={{position: "relative"}}>
                <Typography.Paragraph>
                    Each merchant represents a seperate workspace with its own payments and settings.
                    <br /> You can create as many merchants as you want. <br />
                    <Typography.Text strong>Note: </Typography.Text>
                    Other menu items will appear after you click “Save settings”
                </Typography.Paragraph>
                {merchants?.map((merchant, index) => (
                    <CustomCard
                        key={merchant.id}
                        title={merchant.name}
                        description={merchant.website}
                        rightIcon={
                            <Space>
                                <Button
                                    disabled={isAnyFormOpen}
                                    type="text"
                                    icon={<EditOutlined />}
                                    onClick={() => onEdit(index)}
                                />
                                <Button
                                    type="text"
                                    danger
                                    disabled={isAnyFormOpen}
                                    icon={<DeleteOutlined />}
                                    onClick={() =>
                                        createConfirmPopup(
                                            "Delete the store",
                                            <span>Are you sure to delete this store?</span>,
                                            () => onConfirmDeleteMerchant(merchant)
                                        )
                                    }
                                />
                            </Space>
                        }
                        isActive={merchant.id === merchantId}
                    />
                ))}
                <SpinWithMask isLoading={isTableLoading} />
                {!isTableLoading && (
                    <>
                        <Button
                            disabled={isEditAddMerchantFormOpened}
                            onClick={() => setIsAddMerchantFormOpened(true)}
                            style={{marginTop: "20px", marginRight: "20px"}}
                        >
                            Create merchant
                        </Button>
                        <Button type="primary" disabled={!merchants.length} onClick={() => navigate("/payments")}>
                            Save settings
                        </Button>
                    </>
                )}

                <DrawerForm
                    title="Edit merchant"
                    isFormOpen={isEditAddMerchantFormOpened}
                    changeIsFormOpen={setIsEditAddMerchantFormOpened}
                    formBody={
                        <MerchantForm
                            isFormSubmitting={isFormSubmitting}
                            activeMerchant={activeMerchant}
                            onCancel={() => setIsEditAddMerchantFormOpened(false)}
                            onFinish={(values: MerchantBase, form: FormInstance<MerchantBase>) =>
                                uploadEditedMerchant(activeMerchant!.id, values, form)
                            }
                        />
                    }
                />
                <DrawerForm
                    title="Creating new merchant"
                    isFormOpen={isAddMerchantFormOpened}
                    changeIsFormOpen={setIsAddMerchantFormOpened}
                    formBody={
                        <MerchantForm
                            isFormSubmitting={isFormSubmitting}
                            onCancel={() => {
                                setIsAddMerchantFormOpened(false);
                            }}
                            onFinish={uploadCreatedMerchant}
                        />
                    }
                />
            </div>
        </PageContainer>
    );
};

export default ManageMerchantsPage;
