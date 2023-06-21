import React from "react";
import ReactDOM from "react-dom/client";
import {BrowserRouter} from "react-router-dom";
import posthog from "posthog-js";
import {posthogConfig} from "./providers/posthog";
import {PostHogProvider} from "posthog-js/react";
import App from "./App";

const strictMode = process.env.NODE_ENV === "production";

// https://posthog.com/docs/libraries/react
// eslint-disable-next-line @typescript-eslint/no-unsafe-argument
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
    (strictMode && (
        <React.StrictMode>
            <PostHogProvider client={posthog}>
                <BrowserRouter basename={determineBasename()}>
                    <App />
                </BrowserRouter>
            </PostHogProvider>
        </React.StrictMode>
    )) || (
        <PostHogProvider client={posthog}>
            <BrowserRouter basename={determineBasename()}>
                <App />
            </BrowserRouter>
        </PostHogProvider>
    )
);
