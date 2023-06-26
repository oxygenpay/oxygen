import apiRequest from "src/utils/api-request";
import {User, UserCreateForm, AuthProvider} from "src/types";
import withApiPath from "src/utils/with-api-path";

const authProvider = {
    async getCookie(): Promise<string> {
        const response = await apiRequest.get(withApiPath(`/auth/csrf-cookie`));
        const csrf = response.headers["x-csrf-token"];

        apiRequest.defaults.headers.common["x-csrf-token"] = csrf;
        return csrf ?? "";
    },

    async createUser(user: UserCreateForm): Promise<void> {
        await apiRequest.post(withApiPath(`/auth/login`), user);
        return;
    },

    async getProviders(): Promise<AuthProvider[]> {
        const response = await apiRequest.get(withApiPath(`/auth/provider`));
        return response.data?.providers;
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
