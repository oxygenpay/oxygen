import axios, {AxiosError} from "axios";
import {RenderErrorAlert} from "src/components/ErrorAlert";

const apiRequest = axios.create({
    baseURL: import.meta.env.VITE_BACKEND_HOST,
    headers: {
        "Content-Type": "application/json",
        "Cache-Control": "no-cache",
        Accept: "application/json"
    },
    withCredentials: true
});

apiRequest.interceptors.response.use(undefined, function (error: Error | AxiosError) {
    RenderErrorAlert(error.message);
    return Promise.reject(error);
});

export default apiRequest;
