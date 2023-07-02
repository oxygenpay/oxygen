// import React from "react";
import ReactDOM from "react-dom/client";
import {BrowserRouter} from "react-router-dom";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import posthog from "posthog-js";
import {PostHogProvider} from "posthog-js/react";
import {posthogConfig} from "./providers/posthog";
import App from "./app";

const queryClient = new QueryClient();

// https://posthog.com/docs/libraries/react
// eslint-disable-next-line @typescript-eslint/no-unsafe-argument
// noinspection DuplicatedCode
if (posthogConfig.apiKey) {
    posthog.init(posthogConfig.apiKey, posthogConfig.options);
}

function determineBasename(): string {
    const rootPath = import.meta.env.VITE_ROOTPATH as string;

    if (rootPath.length === 0 || rootPath === "/") {
        return "/";
    }

    if (rootPath.endsWith("/")) {
        return rootPath.slice(0, -1);
    }

    return rootPath;
}

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
    // TODO: Return when https://github.com/ant-design/pro-components/issues/6264 will be fixed
    // <React.StrictMode>
    <PostHogProvider client={posthog}>
        <BrowserRouter basename={determineBasename()}>
            <QueryClientProvider client={queryClient}>
                <App />
            </QueryClientProvider>
        </BrowserRouter>
    </PostHogProvider>
    // </React.StrictMode>
);
