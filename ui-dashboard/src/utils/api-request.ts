import axios, {AxiosError} from "axios";
import {notification} from "antd";
import {ErrorResponse} from "src/types";
import authProvider from "src/providers/auth-provider";

const apiRequest = axios.create({
    baseURL: import.meta.env.VITE_BACKEND_HOST,
    headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-cache",
        Accept: "application/json"
    },
    withCredentials: true
});

apiRequest.interceptors.response.use(undefined, async (error: AxiosError) => {
    if (!error.response) {
        return;
    }

    if (error.response.status === 400) {
        const response: ErrorResponse = error.response.data as ErrorResponse;

        if (!response?.errors) {
            return;
        }

        const errors = response.errors.length
            ? response.errors
                  .map((item) => {
                      return item.message;
                  })
                  .join(", ")
            : "";

        if (response.status === "validation_error") {
            notification.error({
                message: response.message,
                description: !errors ? "Validation error" : "Validation error: " + errors + ".",
                placement: "bottomRight"
            });
        } else {
            notification.error({
                message: response.message,
                description: !errors ? "Validation error" : "Got the following errors: " + errors + ".",
                placement: "bottomRight"
            });
        }
    } else if (error.response.status === 403) {
        try {
            const newToken = await authProvider.getCookie();

            if (error.config?.headers && newToken.length) {
                error.config.headers["x-csrf-token"] = newToken;
                return axios.request(error.config);
            }
        } catch (e) {
            console.error("Ocurred a error: ", e);
        }
    } else if (error.response.status !== 401) {
        notification.error({
            message: "Something went wrong",
            description: error.message,
            placement: "bottomRight"
        });
    }

    return Promise.reject(error);
});

export default apiRequest;
