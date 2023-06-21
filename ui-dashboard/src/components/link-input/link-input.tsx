import * as React from "react";
import {Form, Select, FormRule, Input} from "antd";

interface Props {
    placeholder: string;
    label: string;
    name: string;
    required: boolean;
}

const urlPattern = new RegExp(
    "^(https?:\\/\\/)?((([a-z\\d]([a-z\\d-]*[a-z\\d])*)\\.)+[a-z]{2,}|((\\d{1,3}\\.){3}\\d{1,3}))(\\:\\d+)?(\\/[-a-z\\d%_.~+]*)*(\\?[;&a-z\\d%_.~+=-]*)?(\\#[-a-z\\d_]*)?$",
    "i"
);

const linkPrefix = "https://";

const LinkInput: React.FC<Props> = (props: Props) => {
    const validateLink = async (rule: FormRule, value: string): Promise<void> => {
        if (!value && !props.required) {
            return Promise.resolve();
        }

        if (urlPattern.test(linkPrefix + value)) {
            return Promise.resolve();
        }

        return Promise.reject();
    };

    return (
        <Form.Item
            label={props.label}
            name={props.name}
            rules={[
                {
                    required: props.required,
                    validator: validateLink,
                    message: "Incorrect url value"
                }
            ]}
            validateFirst
            validateTrigger={["onChange", "onBlur"]}
            style={{width: 300}}
            required={props.required}
        >
            <Input
                addonBefore={
                    <Select
                        defaultValue="https://"
                        options={[{value: "https://", label: "https://"}]}
                        className="withdraw-form__currency-selection"
                        disabled
                        showArrow={false}
                    />
                }
                placeholder={props.placeholder}
            />
        </Form.Item>
    );
};

export default LinkInput;
