// import React from "react";
import ReactDOM from "react-dom/client";
import {BrowserRouter} from "react-router-dom";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import posthog from "posthog-js";
import config from "src/config.json";
import {PostHogProvider} from "posthog-js/react";
import App from "./app";

const queryClient = new QueryClient();

// https://posthog.com/docs/libraries/react
// eslint-disable-next-line @typescript-eslint/no-unsafe-argument
posthog.init(config.posthog.apiKey, config.posthog.options);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
    // TODO: Return when https://github.com/ant-design/pro-components/issues/6264 will be fixed
    // <React.StrictMode>
    <PostHogProvider client={posthog}>
        <BrowserRouter>
            <QueryClientProvider client={queryClient}>
                <App />
            </QueryClientProvider>
        </BrowserRouter>
    </PostHogProvider>
    // </React.StrictMode>
);
