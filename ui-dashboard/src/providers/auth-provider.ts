import apiRequest from "src/utils/api-request";
import {User} from "src/types";
import withApiPath from "src/utils/with-api-path";

const authProvider = {
    async getCookie(): Promise<string> {
        const response = await apiRequest.get(withApiPath(`/auth/csrf-cookie`));
        const csrf = response.headers["x-csrf-token"];

        apiRequest.defaults.headers.common["x-csrf-token"] = csrf;
        return csrf ?? "";
    },

    async getMe(): Promise<User> {
        const response = await apiRequest.get(withApiPath(`/auth/me`));
        return response.data;
    },

    async logout(): Promise<void> {
        const response = await apiRequest.post(withApiPath(`/auth/logout`));
        return response.data;
    }
};

export default authProvider;
